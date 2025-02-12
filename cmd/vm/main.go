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
	Project       string `json:"project"`
	Zone          string `json:"zone"`
	Configuration string `json:"configuration"`
	Name          string `json:"name"`
	Alias         string `json:"alias"`
}

type Instance struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`

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
		fmt.Println("  m, edit VM     edit VM instance")
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
		"--project %s --zone %s --configuration %s --format json",
		vm.Name, vm.Project, vm.Zone, vm.Configuration)
	ext.Check(json.Unmarshal(ext.Capture(cmd, echo), &i))
	return &i
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
	vm := &VM{}
	home, err := os.UserHomeDir()
	if err != nil {
		ext.Die("error getting home directory: %s", err)
	}
	b, err := os.ReadFile(home + "/" + meVM)
	if err != nil {
		ext.Die("error reading %s: %s", meVM, err)
	}
	ext.Check(json.Unmarshal(b, vm))
	fmt.Println("vm", ext.Color(vm.Name, c.Magenta))
	return vm
}

func infoCmd() {
	vm := vmInfo()
	i := instanceInfo(vm, true)
	printInstance(vm, i)
}

func printInstance(vm *VM, i *Instance) {
	fmt.Println()
	fmt.Println("Name:   ", ext.Color(i.Name, c.Cyan))
	fmt.Println("Disk:   ", ext.Color(i.Disks[0].DiskSizeGb+"GB", c.Cyan))
	fmt.Println("Email:  ", ext.Color(i.ServiceAccounts[0].Email, c.Cyan))
	fmt.Println("IP:     ", ext.Color(i.NetworkInterfaces[0].AccessConfigs[0].NatIP, c.Cyan))
	fmt.Println("Status: ", ext.Color(i.Status, c.White))
	fmt.Println()
	fmt.Println("Link:   ", instanceLink(vm))
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
		string(ext.Capture(ssh+"df -h /dev/sda1", false))),
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
	cmd := fmt.Sprintf(
		"gcloud compute instances list --project %s --configuration %s",
		vm.Project, vm.Configuration)
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
		"gcloud compute instances start %s "+
		"--project %s --zone %s --configuration %s",
		vm.Name, vm.Project, vm.Zone, vm.Configuration)
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
		"gcloud compute instances stop %s "+
		"--project %s --zone %s --configuration %s",
		vm.Name, vm.Project, vm.Zone, vm.Configuration)
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
