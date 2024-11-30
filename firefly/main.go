package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

const (
	NAME = "firefly"
	PATH = "/bin:/sbin:/usr/bin:/usr/sbin"
)

type MountPoint struct {
	Source string
	FsType string
	Path   string
	Flags  uintptr
	Data   string
}

var mountPointsBoot = []*MountPoint{
	{
		Source: "proc",
		FsType: "proc",
		Path:   "/proc",
		Flags:  0,
	},
	{
		Source: "none",
		FsType: "sysfs",
		Path:   "/sys",
		Flags:  0,
	},
	{
		Source: "dev",
		FsType: "devtmpfs",
		Path:   "/dev",
		Flags:  0,
	},
	{
		Source: "devpts",
		FsType: "devpts",
		Path:   "/dev/pts",
		Flags:  0,
	},
}

var mountPoints = []*MountPoint{
	{
		Source: "proc",
		FsType: "proc",
		Path:   "/proc",
		Flags:  0,
	},
	{
		Source: "none",
		FsType: "sysfs",
		Path:   "/sys",
		Flags:  0,
	},
	{
		Source: "cgroup2",
		FsType: "cgroup2",
		Path:   "/sys/fs/cgroup",
		Flags:  0,
	},
	{
		Source: "dev",
		FsType: "devtmpfs",
		Path:   "/dev",
		Flags:  0,
	},
	{
		Source: "devpts",
		FsType: "devpts",
		Path:   "/dev/pts",
		Flags:  0,
	},
	{
		Source: "mqueue",
		FsType: "mqueue",
		Path:   "/dev/mqueue",
		Flags:  0,
	},
	{
		Source: "shm",
		FsType: "tmpfs",
		Path:   "/dev/shm",
		Flags:  0,
	},
	{
		Source: "/dev/fd",
		FsType: "fdfs",
		Path:   "/dev/fd",
		Flags:  0,
	},
	{
		Source: "run",
		FsType: "tmpfs",
		Path:   "/run",
		Flags:  0,
	},
	{
		Source: "tmp",
		FsType: "tmpfs",
		Path:   "/tmp",
		Flags:  0,
	},
}

func mount(mountPoint *MountPoint, root string) error {
	log.Printf("Mounting %s on %s", mountPoint.FsType, mountPoint.Path)
	realMountPoint := filepath.Join(root, mountPoint.Path)
	// Make dir if it doesn't exist
	err := os.MkdirAll(realMountPoint, 0755)
	if err != nil {
		return err
	}

	err = syscall.Mount(mountPoint.Source, realMountPoint, mountPoint.FsType, mountPoint.Flags, "")
	if err != nil {
		return err
	}

	return nil
}

func mountAll(points []*MountPoint, root string) {
	for _, mountPoint := range points {
		mount(mountPoint, root)
	}
}

func unmountAll(points []*MountPoint) {
	for _, mountPoint := range points {
		unmount(mountPoint.Path)
	}
}

func unmount(path string) {
	log.Printf("Unmounting %s", path)
	syscall.Unmount(path, 0)
}

func runShell() {
	log.Println("Starting shell")

	cmd := exec.Command("/bin/sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to run shell: %s", err)
	}

	log.Println("Shutting down")
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func setHostName(hostname string) {
	err := os.WriteFile("/etc/hostname", []byte(hostname), 0644)
	if err != nil {
		log.Fatalf("Failed to set hostname: %s", err)
	}

	// update /etc/hosts
	hosts, err := os.ReadFile("/etc/hosts")
	if err != nil {
		log.Fatalf("Failed to read /etc/hosts: %s", err)
	}
	// add hostname to /etc/hosts
	hosts = append(hosts, fmt.Sprintf("127.0.0.1\t%s\n", hostname)...)
	err = os.WriteFile("/etc/hosts", hosts, 0644)
	if err != nil {
		log.Fatalf("Failed to update /etc/hosts: %s", err)
	}

	// syscall
	err = syscall.Sethostname([]byte(hostname))
	if err != nil {
		log.Fatalf("Failed to set hostname: %s", err)
	}
}

func setDns(dns string) {

	// in some distros, /etc/resolv.conf is a symlink to /run/resolv.conf, so we need unlink first
	os.Remove("/etc/resolv.conf")

	err := os.WriteFile("/etc/resolv.conf", []byte(fmt.Sprintf("nameserver %s\n", dns)), 0644)
	if err != nil {
		log.Printf("Failed to set dns: %s", err)
	}
}

func setStaticIp(data string) error {
	// ip/mask:gw
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid data format")
	}

	ip := parts[0]
	gw := parts[1]

	var interfaceIndex = -1
	// find network interface
	interfaces, err := netlink.LinkList()
	if err != nil {
		return err
	}
	if len(interfaces) == 0 {
		return fmt.Errorf("no network interfaces found")
	}
	i := 0
	// must start with eth or en
	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Attrs().Name, "eth") || strings.HasPrefix(iface.Attrs().Name, "en") {
			interfaceIndex = i
			break
		}
		i++
	}

	if interfaceIndex < 0 {
		return fmt.Errorf("no network interfaces found")
	}

	// set ip
	addr, err := netlink.ParseAddr(ip)
	if err != nil {
		return err
	}
	err = netlink.AddrAdd(interfaces[interfaceIndex], addr)
	if err != nil {
		return err
	}

	err = netlink.LinkSetUp(interfaces[interfaceIndex])
	if err != nil {
		return err
	}

	// set gateway
	err = netlink.RouteAdd(&netlink.Route{
		LinkIndex: interfaces[interfaceIndex].Attrs().Index,
		Gw:        net.ParseIP(gw),
	})
	if err != nil {
		return err
	}

	return nil
}

func setupLoopback() {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return
	}
	err = netlink.LinkSetUp(lo)
	if err != nil {
		return
	}
}

func getKernelCmdLineArgs() map[string]string {
	cmdlineFile := "/proc/cmdline"

	cmdline, err := os.ReadFile(cmdlineFile)
	if err != nil {
		log.Fatal(err)
	}

	parts := strings.Split(string(cmdline), " ")

	cmdlineArgs := make(map[string]string)
	for _, part := range parts {
		if strings.Contains(part, "=") {
			kv := strings.Split(part, "=")
			cmdlineArgs[kv[0]] = strings.TrimSpace(kv[1])
		} else {
			cmdlineArgs[part] = ""
		}
	}

	return cmdlineArgs
}

func main() {
	if os.Getpid() != 1 {
		panic("Program must be run as PID 1")
	}

	log.Print(logo)
	log.Printf("Starting %s", NAME)
	log.Printf("PATH=%s", PATH)
	os.Setenv("PATH", PATH)
	mountAll(mountPointsBoot, "/")

	cmdline := getKernelCmdLineArgs()
	log.Printf("cmdline=%v", cmdline)

	// Mount /
	rootDevice, ok := cmdline["root"]
	if !ok {
		log.Fatal("No root device specified")
	}
	rootfstype, ok := cmdline["rootfstype"]
	if !ok {
		rootfstype = "ext4"
	}

	log.Printf("root=%s", rootDevice)
	startTime := time.Now()
	// wait rootdevice ready
	for {
		var err error
		if _, err = os.Stat(rootDevice); err == nil {
			break
		}
		time.Sleep(time.Microsecond * 500)
		if time.Since(startTime).Seconds() > 10 {
			log.Printf("Timeout waiting for root device")
			runShell()
			break
		}
	}

	err := mount(&MountPoint{
		Source: rootDevice,
		FsType: rootfstype,
		Path:   "/mnt",
		Flags:  0,
	}, "/")
	if err != nil {
		log.Printf("Failed to mount root: %s", err)
		runShell()
	}

	unmountAll(mountPointsBoot)

	// switch root
	log.Print("Switching root")
	if err := syscall.Chroot("/mnt"); err != nil {
		log.Printf("Failed to chroot: %s", err)
		runShell()
	}
	log.Print("Switched root")
	// chdir
	if err := syscall.Chdir("/"); err != nil {
		log.Printf("Failed to chdir: %s", err)
	}
	// Re mount
	mountAll(mountPoints, "/")

	hostname, ok := cmdline["hostname"]
	if !ok {
		hostname = "firefly"
	}
	setHostName(hostname)
	dns, ok := cmdline["dns"]
	if !ok {
		dns = "8.8.8.8"
	}
	setDns(dns)
	ip, ok := cmdline["ip"]
	if ok {
		err := setStaticIp(ip)
		if err != nil {
			log.Printf("Failed to set static ip: %s", err)
		}
	} else {
		log.Printf("No static ip specified! guest will not be able to connect to the internet")
	}
	setupLoopback()

	// Run endpoint
	endpoint, ok := cmdline["endpoint"]
	if !ok {
		endpoint = "/bin/sh"
	}

	cmd := exec.Command(endpoint)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start endpoint: %s", err)
	}
	err = cmd.Wait()

	log.Println("Shutting down...")
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	if err != nil {
		fmt.Printf("Endpoint process exited with error: %v\n", err)
	} else {
		fmt.Println("Endpoint process exited successfully")
	}

}
