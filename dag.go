package main

import (
	"io/ioutil"
	"log"
	"strings"

	"github.com/ice2heart/proxyu_client/common"

	"gopkg.in/yaml.v2"
)

// DAGYAML struct for parsing
type DAGYAML struct {
	Didgraph []struct {
		Key         string
		Mime        string
		Description string
		Children    []string `yaml:",flow"`
	} `yaml:",flow"`
}

var (
	daggraph DAGYAML
	graph    map[[16]byte][][16]byte
)

// ParseDAGYML tree
func ParseDAGYML(path *string) {
	data, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Panic(err)
	}
	daggraph = DAGYAML{}
	err = yaml.Unmarshal([]byte(data), &daggraph)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	graph = make(map[[16]byte][][16]byte)
	for k := range daggraph.Didgraph {
		var rawUUID [16]byte
		copy(rawUUID[:], common.UUID2bytes(daggraph.Didgraph[k].Key))
		for _, ch := range daggraph.Didgraph[k].Children {
			var child [16]byte
			copy(child[:], common.UUID2bytes(ch))
			graph[rawUUID] = append(graph[rawUUID], child)
		}
	}
}

// ParseDAGDevYML prepare map for easy access
func ParseDAGDevYML(path *string) (result map[string][]byte) {
	data, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Panic(err)
	}
	result = make(map[string][]byte)
	m := make(map[string]string)

	err = yaml.Unmarshal([]byte(data), &m)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	for k, v := range m {
		result[strings.ToUpper(v)] = common.UUID2bytes(k)
	}
	return
}

// GetDAGChildren return slice of children
func GetDAGChildren(ID *[16]byte) (children [][16]byte) {
	children = graph[*ID]
	return
}
