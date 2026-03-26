package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"gcp/lib/completion/zsh"
	"gcp/lib/ext"

	"github.com/briandowns/spinner"
	c "github.com/logrusorgru/aurora/v4"
)

type VM struct {
	Project string `json:"project"`
	Zone    string `json:"zone"`
	GAC     string `json:"gac"`
	Name    string `json:"name"`
	Disk    string `json:"disk"`
	Alias   string `json:"alias"`
}

type VMConfig struct {
	Default string `json:"default"`
	VMs     []VM   `json:"vms"`
}

var fMachine = flag.String("m", "", "VM name or alias")
var fExtra = flag.Bool("x", false, "show extra info (ssh keys)")

type Instance struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	MachineType string `json:"machineType"`

	Disks []struct {
		DiskSizeGb string `json:"diskSizeGb"`
	} `json:"disks"`

	NetworkInterfaces []struct {
		AccessConfigs []struct {
			NatIP string `json:"natIP"`
		} `json:"accessConfigs"`
	} `json:"networkInterfaces"`

	ServiceAccounts []struct {
		Email string `json:"email"`
	} `json:"serviceAccounts"`

	Tags struct {
		Items []string `json:"items"`
	} `json:"tags"`

	GuestAccelerators []struct {
		AcceleratorType  string `json:"acceleratorType"`
		AcceleratorCount int    `json:"acceleratorCount"`
	} `json:"guestAccelerators"`

	Metadata struct {
		Items []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"items"`
	} `json:"metadata"`
}

type MachineType struct {
	GuestCpus int `json:"guestCpus"`
	MemoryMb  int `json:"memoryMb"`
}

func main() {
	zsh.Completion(CompletionRoot)

	flag.Usage = func() {
		fmt.Printf("usage: %s command [command]...", os.Args[0])
		fmt.Println("commands:")
		fmt.Println("  i, info        VM instance info (default)")
		fmt.Println("  p, ping        ping VM")
		fmt.Println("  l, list        list VM instances")
		fmt.Println("  h, hosts       list ~/" + sshConfig + " hosts and IPs")
		fmt.Println("  e, edit        edit ~/" + sshConfig)
		fmt.Println("  m, edit VM     edit VM config")
		fmt.Println("  c, configure    update ~/" + sshConfig)
		fmt.Println("  up, start      start vm")
		fmt.Println("  down, stop     stop vm")
		fmt.Println("  completion     generate completion script")
		fmt.Println()
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"i"}
	}

	for _, cmd := range args {
		switch cmd {
		case "i", "info":
			infoCmd()
		case "p", "ping":
			pingCmd()
		case "l", "list":
			listCmd()
		case "up", "start":
			startCmd()
		case "down", "stop":
			stopCmd()
		case "c", "configure":
			configureCmd(vmInfo(), nil)
		case "e", "edit":
			editCmd()
		case "m", "vm":
			vmEditCmd()
		case "h", "hosts":
			sshHostsCmd()
		case "completion":
			zsh.Script()
		default:
			ext.Die("unknown command: %s", cmd)
		}
	}
}

func instanceInfo(vm *VM, echo bool) *Instance {
	i := Instance{}
	cmd := fmt.Sprintf(""+
		"gcloud compute instances describe %s "+
		"--project %s --zone %s --format json",
		vm.Name, vm.Project, vm.Zone)
	ext.Check(json.Unmarshal(ext.Capture(cmd, echo), &i))
	return &i
}

func machineTypeName(i *Instance) string {
	parts := strings.Split(i.MachineType, "/")
	return parts[len(parts)-1]
}

func machineTypeInfo(vm *VM, i *Instance) *MachineType {
	mt := &MachineType{}
	cmd := fmt.Sprintf("gcloud compute machine-types describe %s "+
		"--project %s --zone %s --format json",
		machineTypeName(i), vm.Project, vm.Zone)
	ext.Check(json.Unmarshal(ext.Capture(cmd, false), mt))
	return mt
}

func instanceLink(vm *VM) string {
	link := fmt.Sprintf(
		"%s/compute/instancesDetail/zones/%s/instances/%s?project=%s",
		ext.ConsoleURL, vm.Zone, vm.Name, vm.Project)
	return link
}

var vmi *VM

const meVM = "me/vm.json"

func vmInfo() *VM {
	if vmi != nil {
		return vmi
	}
	home, err := os.UserHomeDir()
	if err != nil {
		ext.Die("error getting home directory: %s", err)
	}
	b, err := os.ReadFile(home + "/" + meVM)
	if err != nil {
		ext.Die("error reading %s: %s", meVM, err)
	}

	config := &VMConfig{}
	ext.Check(json.Unmarshal(b, config))

	name := *fMachine
	if name == "" {
		name = config.Default
	}

	for i := range config.VMs {
		if config.VMs[i].Name == name || config.VMs[i].Alias == name {
			vmi = &config.VMs[i]
			break
		}
	}
	if vmi == nil {
		ext.Die("VM %q not found in %s", name, meVM)
	}

	fmt.Println("vm", ext.Color(vmi.Name, c.Magenta))

	if vmi.GAC == "" {
		ext.Die("gac not set in %s", meVM)
	}
	gac := os.ExpandEnv(vmi.GAC)
	ext.SetVariable("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", gac)
	return vmi
}

func infoCmd() {
	vm := vmInfo()
	i := instanceInfo(vm, true)
	printInstance(vm, i)
}

func onOff(tags []string, tag string) string {
	for _, t := range tags {
		if t == tag {
			return "on"
		}
	}
	return "off"
}

func printInstance(vm *VM, i *Instance) {
	mt := machineTypeInfo(vm, i)
	fmt.Println()
	fmt.Println("Name:   ", ext.Color(i.Name, c.Cyan))
	fmt.Println("Type:   ", ext.Color(machineTypeName(i), c.Cyan))
	fmt.Println("CPU:    ", ext.Color(fmt.Sprintf("%d vCPU", mt.GuestCpus), c.Cyan))
	fmt.Println("Memory: ", ext.Color(fmt.Sprintf("%.1f GB", float64(mt.MemoryMb)/1024), c.Cyan))
	if len(i.GuestAccelerators) > 0 {
		for _, gpu := range i.GuestAccelerators {
			parts := strings.Split(gpu.AcceleratorType, "/")
			name := parts[len(parts)-1]
			fmt.Println("GPU:    ", ext.Color(fmt.Sprintf("%dx %s", gpu.AcceleratorCount, name), c.Cyan))
		}
	} else {
		fmt.Println("GPU:    ", ext.Color("none", c.Cyan))
	}
	fmt.Println("Disk:   ", ext.Color(i.Disks[0].DiskSizeGb+" GB", c.Cyan))
	fmt.Println("IP:     ", ext.Color(i.NetworkInterfaces[0].AccessConfigs[0].NatIP, c.Cyan))
	fmt.Println("Email:  ", ext.Color(i.ServiceAccounts[0].Email, c.Cyan))
	fmt.Println("HTTP:   ", ext.Color(onOff(i.Tags.Items, "http-server"), c.Cyan))
	fmt.Println("HTTPS:  ", ext.Color(onOff(i.Tags.Items, "https-server"), c.Cyan))
	if len(i.Tags.Items) > 0 {
		fmt.Println("Tags:   ", ext.Color(strings.Join(i.Tags.Items, ", "), c.Cyan))
	}
	fmt.Println("Status: ", ext.Color(i.Status, c.White))
	fmt.Println()
	fmt.Println("Link:   ", instanceLink(vm))

	if *fExtra {
		fmt.Println()
		fmt.Println(ext.Color("SSH Keys:", c.White))
		for _, item := range i.Metadata.Items {
			if item.Key != "ssh-keys" {
				continue
			}
			for _, line := range strings.Split(strings.TrimSpace(item.Value), "\n") {
				// format: username:key-type base64 comment...
				user, rest, _ := strings.Cut(line, ":")
				fields := strings.Fields(rest)
				keyType := ""
				comment := ""
				if len(fields) > 0 {
					keyType = fields[0]
				}
				key := ""
				if len(fields) > 1 {
					b := fields[1]
					if len(b) > 16 {
						key = b[:8] + "..." + b[len(b)-8:]
					} else {
						key = b
					}
				}
				if len(fields) > 2 {
					comment = strings.Join(fields[2:], " ")
				}
				fmt.Println(" ", ext.Color(user, c.Cyan), keyType, key, comment)
			}
		}
	}

	fmt.Println()
}

func pingCmd() {
	vm := vmInfo()
	i := instanceInfo(vm, true)

	if i.Status != "RUNNING" {
		ext.Die("%q is not running", vm.Name)
	}

	ssh := "ssh " + vm.Alias + " "

	fmt.Print(ext.Color("✔️ ", c.Blue))
	ext.Quiet(ssh+"hostname", 10)

	fmt.Print(ext.Color("✔️ ", c.Blue))
	ext.Quiet(ssh + "uname -a")

	fmt.Print(ext.Color("✔️ ", c.Blue))
	fmt.Println(strings.TrimSpace(string(ext.Capture(ssh+"uptime", false))))

	fmt.Print(ext.Color("✔️ ", c.Blue))
	fmt.Println(strings.ReplaceAll(strings.TrimSpace(
		string(ext.Capture(ssh+"df -h "+vm.Disk, false))),
		"\n", ext.Color("\n✔️ ", c.Blue)))

	fmt.Print(ext.Color("✔️ ", c.Blue))
	fmt.Println(strings.ReplaceAll(strings.TrimSpace(
		string(ext.Capture(ssh+"lsmem --summary=only", false))),
		"\n", ext.Color("\n✔️ ", c.Blue)))

	fmt.Print(ext.Color("✔️ ", c.Blue))
	ext.Quiet(ssh + "curl -s https://api.ipify.org")

	fmt.Println()
}

func listCmd() {
	vm := vmInfo()
	cmd := fmt.Sprintf("gcloud compute instances list --project %s", vm.Project)
	ext.Run(cmd)
}

func awaitInstanceStatus(instance *Instance, status string) {
	vm := vmInfo()

	fmt.Println("\n> status", ext.Color(instance.Status, c.Blue))

	if instance.Status == status {
		return
	}

	s := spinner.New(
		spinner.CharSets[14], 100*time.Millisecond,
		spinner.WithHiddenCursor(false),
	)
	s.Start()
	for instance.Status != status {
		time.Sleep(2 * time.Second)
		*instance = *instanceInfo(vm, false)
	}
	s.Stop()
	fmt.Println("> status", ext.Color(instance.Status, c.White))
}

func startCmd() {
	vm := vmInfo()
	instance := instanceInfo(vm, true)

	if instance.Status == "RUNNING" {
		printInstance(vm, instance)
		ext.Die(fmt.Sprintf("%q is already running", vm.Name))
	}

	cmd := fmt.Sprintf(""+
		"gcloud compute instances start %s --project %s --zone %s",
		vm.Name, vm.Project, vm.Zone)
	ext.Run(cmd)

	awaitInstanceStatus(instance, "RUNNING")
	fmt.Println(instance.NetworkInterfaces[0].AccessConfigs[0].NatIP)

	configureCmd(vm, instance)
	pingCmd()
}

func stopCmd() {
	vm := vmInfo()
	instance := instanceInfo(vm, true)

	if instance.Status != "RUNNING" {
		printInstance(vm, instance)
		ext.Die("%q is not running", vm.Name)
	}

	if !ext.Confirm(fmt.Sprintf("stop %q", vm.Name)) {
		return
	}

	cmd := fmt.Sprintf(""+
		"gcloud compute instances stop %s --project %s --zone %s",
		vm.Name, vm.Project, vm.Zone)
	ext.Run(cmd)

	awaitInstanceStatus(instance, "TERMINATED")
}

func configureCmd(vm *VM, instance *Instance) {
	confirm := instance == nil
	if instance == nil {
		instance = instanceInfo(vm, false)
	}

	if instance.Status != "RUNNING" {
		ext.Die("%q is not running", vm.Name)
	}

	newIP := instance.NetworkInterfaces[0].AccessConfigs[0].NatIP

	home, err := os.UserHomeDir()
	if err != nil {
		ext.Die("error getting home directory: %s", err)
	}
	sshConfig := home + "/" + sshConfig

	b, err := os.ReadFile(sshConfig)
	if err != nil {
		ext.Die("error reading ssh config: %s", err)
	}

	backupDir := home + "/Downloads"
	backup := backupDir + "/ssh-config.vm-" + time.Now().Format("20060102-150405") + ".txt"
	err = os.WriteFile(backup, b, 0o644)
	if err != nil {
		ext.Die("error writing backup: %s", err)
	}

	t := string(b)
	lines := strings.Split(t, "\n")

	updated := false
	for i, line := range lines {
		if line != "Host "+vm.Alias {
			continue
		}
		updated = true

		hostName := strings.TrimSpace(lines[i+1])
		if !strings.HasPrefix(hostName, "HostName") {
			ext.Die("expected HostName after %q at line %d", vm.Alias, i+1)
		}
		fields := strings.Fields(hostName)
		if len(fields) < 2 {
			ext.Die("expected HostName value after %q at line %d", vm.Alias, i+1)
		}
		previousIP := fields[1]

		if previousIP == newIP {
			fmt.Printf("✔️ %s already updated: %s\n", vm.Alias, newIP)
		} else {
			fmt.Println(ext.Color("- "+hostName, c.Red))

			update := "HostName " + newIP + " # previous " + previousIP +
				", updated " + time.Now().Format("2006-01-02 15:04:05")
			fmt.Println(ext.Color("+ "+update, c.White))

			if confirm {
				if !ext.Confirm("update " + vm.Alias + " in " + sshConfig) {
					return
				}
			}

			lines[i+1] = "  " + update

			err = os.WriteFile(sshConfig, []byte(strings.Join(lines, "\n")), 0o644)
			if err != nil {
				ext.Die("error writing ssh config: %s", err)
			}
			fmt.Println("updated", sshConfig)
		}
		break
	}
	if !updated {
		ext.Die("%q not found in ", vm.Alias, sshConfig)
	}
}

const sshConfig = ".ssh/config"

func editCmd() {
	editor(sshConfig)
}

func vmEditCmd() {
	editor(meVM)
}

func editor(filename string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		ext.Die("error getting home directory: %s", err)
	}
	filename = home + "/" + filename
	cmd := exec.Command(editor, filename)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	ext.Check(cmd.Run())
}

func sshHostsCmd() {
	home, err := os.UserHomeDir()
	if err != nil {
		ext.Die("error getting home directory: %s", err)
	}
	b, err := os.ReadFile(home + "/" + sshConfig)
	if err != nil {
		ext.Die("error reading ssh config: %s", err)
	}

	t := string(b)
	lines := strings.Split(t, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "Host ") {
			host := strings.TrimSpace(strings.TrimPrefix(line, "Host "))
			if host == "*" {
				continue
			}
			hostName := strings.TrimSpace(lines[i+1])
			if !strings.HasPrefix(hostName, "HostName") {
				ext.Die("expected HostName after %q at line %d", host, i+1)
			}
			fields := strings.Fields(hostName)
			if len(fields) < 2 {
				ext.Die("expected HostName value after %q at line %d", host, i+1)
			}
			ip := fields[1]
			fmt.Println(host, ip)
		}
	}
}

var CompletionRoot = zsh.Args(
	zsh.NewArg("i:info", "VM instance info (default)"),
	zsh.NewArg("p:ping", "ping VM"),
	zsh.NewArg("l:list", "list VM instances"),
	zsh.NewArg("h:hosts", "list ~/"+sshConfig+" hosts and IPs"),
	zsh.NewArg("c:configure", "update ~/"+sshConfig),
	zsh.NewArg("up:start", "start VM"),
	zsh.NewArg("down:stop", "stop VM"),
	zsh.NewArg("e:edit", "edit ~/"+sshConfig),
	zsh.NewArg("m:vm", "edit VM instance ~/"+meVM),
	zsh.NewArg("completion", "generate completion script"),
)
