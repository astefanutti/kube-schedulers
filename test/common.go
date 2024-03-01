package test

const kwokNode = "kwok.x-k8s.io/node"

const (
	NodesCount = 100
	JobsCount  = 500
)

type NodeType string

var (
	fake   NodeType = "fake"
	sample NodeType = "sample"
)
