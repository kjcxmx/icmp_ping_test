package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type IPing struct {
	Host   string
	Ip     string
	IpType int
	Count  int
	Seq    []string
}

type IPingTask struct {
	OsType string
	Count  string
	IPings []IPing
}

func newTask() *IPingTask {
	//get os and set default value
	return &IPingTask{OsType: getOsType(), Count: "4"}
}

func (i *IPingTask) addTask(ip string) {
	var iping IPing
	//select ip or host, then set ipv4 or ipv6
	if ipaddr := net.ParseIP(ip); ipaddr != nil {
		iping.Ip = ip
		iping.IpType = 6
		if ipaddr.To4() != nil {
			iping.IpType = 4
		}
	} else {
		iping.Host = ip
	}

	i.IPings = append(i.IPings, iping)
}

func (i *IPing) print() {
		if len(i.Seq) > 0 {
			fmt.Println(i.Host, i.Ip, i.Seq)
		}
}

func (i *IPing) pingTest() {
	nwork := "ip4:icmp"
	if i.IpType == 6 {
		nwork = "ip6:ipv6-icmp"
	}

	ip := i.Ip
	if i.Host != "" {
		ip = i.Host
	}

	conn, err := icmp.ListenPacket(nwork, "")
	if err != nil {
		fmt.Println("conn error :", err)
	}
	defer conn.Close()

	dst, err := net.ResolveIPAddr(nwork, ip)
	if err != nil {
		fmt.Println(err)
	}

	for j := 1; j <= i.Count; j++ {
		fmt.Println(j,ip,i.IpType)
		conn.SetDeadline(time.Now().Add(1 * time.Second))
		wm := icmp.Message{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmp.Echo{
				ID: os.Getpid() & 0xffff, Seq: j,
				Data: []byte("HELLO"),
			},
		}

		if i.IpType == 6 {
			wm.Type = ipv6.ICMPTypeEchoRequest
		}

		wb, err := wm.Marshal(nil)
		if err != nil {
			fmt.Println("marshal error :", err)
			continue
		}

		if _, err := conn.WriteTo(wb, dst); err != nil {
			//fmt.Println("write error :", err)
			return
		}

		rb := make([]byte, 300)
		_, raddr, err := conn.ReadFrom(rb)
		if err != nil {
			//fmt.Println("read error :", err)
			return
		}

		if i.Host != "" {
			i.Ip = raddr.String()
		}
		i.Seq = append(i.Seq, strconv.Itoa(j))
		time.Sleep(10*time.Millisecond)
	}
}

func getOsType() string {
	return runtime.GOOS
}

func incIp(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func foreachIp(netCIDR string) (ips []string) {
	ip, ipNet, err := net.ParseCIDR(netCIDR)
	if err != nil {
		fmt.Println("invalid CIDR")
	}

	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); incIp(ip) {
		ips = append(ips, ip.String())
	}
	return
}

var (
	ips []string
	wg  sync.WaitGroup
)

func main() {
	t := flag.String("t", "", "host or ip addr, i.e.: \n-t 127.0.0.1 or -t www.baidu.com")
	i := flag.String("i", "", "ip addr of range, i.e.: \n-i 127.0.0.1/24")
	c := flag.Int("c", 4, "request arp packet count, i.e.: \n-c 4")
	flag.Parse()

	if *t == "" && *i == "" {
		flag.Usage()
		return
	}

	result := make(chan IPing)
	task := newTask()

	if *i != "" {
		ips = foreachIp(*i)
	} else {
		ips = append(ips, *t)
	}

	for _, v := range ips {
		task.addTask(v)
	}

	for _, v := range task.IPings {
		wg.Add(1)
		v.Count = *c
		go func(i IPing) {
			defer wg.Done()
			i.pingTest()
			if len(i.Seq) > 0 {
				result <- i
			}
		}(v)
	}
	go func() {
		for v := range result {
			v.print()
		}
	}()
	wg.Wait()
	time.Sleep(2)
}
