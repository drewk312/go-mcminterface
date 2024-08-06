package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"time"
)

var Settings SettingsType

// Global settings
type SettingsType struct {
	StartIPs           []string
	IPs                []string
	Nodes              []RemoteNode
	IPExpandDepth      int
	ForceQueryStartIPs bool // Forces to query only start ips bypassing PickNodes
	QuerySize          int  // Number of nodes to query, quorum is 50% + 1
}

type RemoteNode struct {
	IP       string
	LastSeen time.Time
	Ping     uint32
}

// load settings from settings.json
func LoadSettings() SettingsType {
	file, err := os.Open("settings.json")
	if err != nil {
		fmt.Println("Error opening settings.json")
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	settings := SettingsType{}
	err = decoder.Decode(&settings)
	if err != nil {
		fmt.Println("Error decoding settings.json")
	}
	return settings
}

// save settings to settings.json
func SaveSettings(settings SettingsType) {
	file, err := os.Create("settings.json")
	if err != nil {
		fmt.Println("Error creating settings.json")
	}
	defer file.Close()
	// format with indentation
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	err = encoder.Encode(settings)

	if err != nil {
		fmt.Println("Error encoding settings.json")
	}
}

// Expand known IPs
func ExpandIPs() {
	// Add start IPs to the settings IPs
	Settings.IPs = append(Settings.IPs, Settings.StartIPs...)
	queriedIPs := make(map[string]bool)

	for i := 0; i < Settings.IPExpandDepth; i++ {
		ips := make([]string, 0)
		ch := make(chan string)

		for _, ip := range Settings.IPs {
			if queriedIPs[ip] {
				continue // Skip already queried IPs
			}
			queriedIPs[ip] = true

			go func(ip string) {
				sd := ConnectToNode(ip)
				if sd.block_num == 0 {
					fmt.Println("Connection failed")
					ch <- ""
					return
				}
				new_ips, err := sd.GetIPList()
				if err != nil {
					fmt.Println("Error:", err)
					ch <- ""
					return
				}
				// Add new IPs to the list if they are not already present
				for _, new_ip := range new_ips {
					found := false
					for _, ip := range ips {
						if new_ip == ip {
							found = true
							break
						}
					}
					if !found {
						ips = append(ips, new_ip)
					}
				}
				ch <- ip
			}(ip)
		}

		timeout := time.After(5 * time.Second)
		for range Settings.IPs {
			select {
			case ip := <-ch:
				if ip != "" {
					ips = append(ips, ip)
				}
			case <-timeout:
				fmt.Println("Timeout")
				return
			}
		}

		Settings.IPs = ips
	}
}

// Benchmark all IPs in the time they take to ConnectToNode
func BenchmarkNodes(n int) {
	ch := make(chan RemoteNode)

	for i := 0; i < len(Settings.IPs); i += n {
		end := i + n
		if end > len(Settings.IPs) {
			end = len(Settings.IPs)
		}
		ips := Settings.IPs[i:end]

		for _, ip := range ips {
			go func(ip string) {
				start := time.Now()
				sd := ConnectToNode(ip)
				ping := time.Since(start)
				if sd.block_num == 0 {
					fmt.Println("Connection failed")
					ping = 10 * time.Second
				}
				// ping in milliseconds
				ch <- RemoteNode{IP: ip, Ping: uint32(ping / time.Millisecond)}
			}(ip)
		}
	}

	timeout := time.After(5 * time.Second) // Set timeout of 5 seconds

	for i := 0; i < len(Settings.IPs); i += n {
		end := i + n
		if end > len(Settings.IPs) {
			end = len(Settings.IPs)
		}
		ips := Settings.IPs[i:end]

		for range ips {
			select {
			case node := <-ch:
				found := false
				for i, n := range Settings.Nodes {
					if n.IP == node.IP {
						Settings.Nodes[i].Ping = (n.Ping*2 + node.Ping) / 3
						Settings.Nodes[i].LastSeen = time.Now()
						found = true
						break
					}
				}
				if !found {
					Settings.Nodes = append(Settings.Nodes, node)
				}
			case <-timeout:
				fmt.Println("Timeout")
				return
			}
		}
	}

	close(ch)
}

// Pick n random nodes from Settings.Nodes
// the probability of picking a node is e**(-ping)
func PickNodes(n int) []RemoteNode {
	// if forcequerystartips is set, return the nodes with ip startip
	if Settings.ForceQueryStartIPs {
		nodes := make([]RemoteNode, 0)
		for _, node := range Settings.Nodes {
			if node.IP == Settings.StartIPs[0] {
				nodes = append(nodes, node)
			}
		}
		return nodes
	}

	if n >= len(Settings.Nodes) {
		return Settings.Nodes
	}

	nodes := make([]RemoteNode, 0)
	for i := 0; i < n; i++ {
		// calculate the sum of e**(-ping) for all nodes
		sum := 0.0
		for _, node := range Settings.Nodes {
			sum += math.Exp(-1 / float64(node.Ping/2))
		}
		// pick a random number between 0 and sum
		r := sum * rand.Float64()
		// find the node that corresponds to the random number
		for _, node := range Settings.Nodes {
			r -= math.Exp(-1 / float64(node.Ping/2))
			if r <= 0 {
				// if it is already in the list, decrease i and continue
				found := false
				for _, n := range nodes {
					if n.IP == node.IP {
						i--
						found = true
						break
					}
				}
				if !found {
					nodes = append(nodes, node)
				}
				break
			}
		}
	}
	return nodes
}

// Query the balance of an address given as hex
func QueryBalance(wots_address string) (uint64, error) {
	wots_addr := WotsAddressFromHex(wots_address)

	// connect to a random node
	nodes := PickNodes(Settings.QuerySize)
	balances := make([]WotsAddress, 0)

	// Ask for result on the same time
	ch := make(chan WotsAddress)

	for _, node := range nodes {
		go func(node RemoteNode) {
			sd := ConnectToNode(node.IP)
			if sd.block_num == 0 {
				fmt.Println("Connection failed")
				ch <- WotsAddress{}
				return
			}
			// get the balance of the wots_addr GetBalance
			balance, err := sd.GetBalance(wots_addr)
			if err != nil {
				fmt.Println("Error:", err)
				ch <- WotsAddress{}
				return
			}
			wots_addr.Amount = balance
			ch <- wots_addr
		}(node)
	}

	timeout := time.After(5 * time.Second) // Set timeout of 5 seconds

	for range nodes {
		select {
		case balance := <-ch:
			if balance.Amount != 0 {
				balances = append(balances, balance)
			}
		case <-timeout:
			fmt.Println("Timeout")
			return 0, fmt.Errorf("timeout")
		}
	}

	close(ch)

	// Calculate the most frequent balance
	counts := make(map[uint64]int)
	for _, balance := range balances {
		counts[balance.Amount]++
	}

	// See if there is a balance that reaches quorum

	max_balance := uint64(0)
	for balance, count := range counts {
		if count >= Settings.QuerySize/2+1 {
			max_balance = balance
			break
		}
	}

	// If no balance reaches quorum, return 0
	if max_balance == 0 {
		return 0, fmt.Errorf("no balance reaches quorum")
	}

	// Save the most frequent balance to the wots_address
	wots_addr.Amount = max_balance
	//fmt.Println("Wots address:", wots_addr)

	return max_balance, nil
}

//
