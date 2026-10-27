package ballo

import (
	"strings"
)

type router struct {
	root    *node
	methods map[string]bool
}

type node struct {
	path     string
	partial  bool // true if this is a parameter segment
	param    string
	children []*node
	handler  Handler
	methods  []string
}

type routeMatch struct {
	handler Handler
	method  string
}

func newRouter() *router {
	return &router{
		root:    &node{},
		methods: make(map[string]bool),
	}
}

func (r *router) add(method, path string, handler Handler) {
	r.methods[method] = true
	n := r.root
	parts := splitPath(path)
	for i, part := range parts {
		found := false
		for _, child := range n.children {
			if child.path == part {
				n = child
				found = true
				break
			}
		}
		if !found {
			newNode := &node{path: part}
			if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
				newNode.partial = true
				newNode.param = part[1:]
			}
			n.children = append(n.children, newNode)
			n = newNode
		}
		if i == len(parts)-1 {
			n.handler = handler
			n.methods = append(n.methods, method)
		}
	}
}

func (r *router) match(path string) *routeMatch {
	parts := splitPath(path)
	return r.matchNode(r.root, parts, 0, make(map[string]string))
}

func (r *router) matchNode(n *node, parts []string, idx int, params map[string]string) *routeMatch {
	if idx == len(parts) {
		if n.handler != nil {
			// Deep copy params
			p := make(map[string]string, len(params))
			for k, v := range params {
				p[k] = v
			}
			return &routeMatch{handler: n.handler, method: n.methods[0]}
		}
		return nil
	}

	part := parts[idx]
	for _, child := range n.children {
		if !child.partial && child.path == part {
			if m := r.matchNode(child, parts, idx+1, params); m != nil {
				return m
			}
		}
		if child.partial {
			if child.param != "" {
				params[child.param] = part
			}
			if m := r.matchNode(child, parts, idx+1, params); m != nil {
				return m
			}
			delete(params, child.param)
		}
	}
	return nil
}

func (r *router) hasPath(path string) bool {
	return r.match(path) != nil
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{""}
	}
	return strings.Split(path, "/")
}
