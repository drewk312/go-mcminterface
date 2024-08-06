package main

import (
	"encoding/hex"
	"fmt"
)

// Resolve tag 01b0ec67eb4e7c25a2aa34d6
func test_resolve_balance() {
	sd := ConnectToNode("192.168.1.70")
	if sd.block_num == 0 {
		fmt.Println("Connection failed")
		return
	}

	tag := []byte{0x01, 0xb0, 0xec, 0x67, 0xeb, 0x4e, 0x7c, 0x25, 0xa2, 0xaa, 0x34, 0xd6}

	addr, err := sd.ResolveTag(tag)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Address:", addr)
	// print the balance
	fmt.Println("Balance:", addr.GetAmount()/1000000000)
	fmt.Println("Block number:", sd.block_num)
}

func test_dl_block() {
	sd := ConnectToNode("192.168.1.70")
	fmt.Println("Block number:", sd.block_num)
	file, err := sd.GetBlockBytes(sd.block_num)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	block := BlockFromBytes(file)
	// print how many transactions are in the block
	fmt.Println("Transactions:", len(block.Body))
}

func test_query_balance() {
	// resolve tag
	sd := ConnectToNode("192.168.1.70")
	tag := []byte{0x01, 0xb0, 0xec, 0x67, 0xeb, 0x4e, 0x7c, 0x25, 0xa2, 0xaa, 0x34, 0xd6}

	addr, err := sd.ResolveTag(tag)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	bal, err := QueryBalance(hex.EncodeToString(addr.Address[:]))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Balance:", bal)

}
