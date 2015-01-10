// Package graph implements generic graph
package graph

type Vertex interface{}

type Edge interface{}

type Graph map[Vertex]map[Vertex]Edge

func New() Graph {
	return make(Graph)
}

func (g Graph) AddVertex(v Vertex) {
	g[v] = make(map[Vertex]Edge)
}

func (g Graph) AddDirected(from, to Vertex, e Edge) {
	if _, ok := g[from]; !ok {
		g.AddVertex(from)
	}
	if _, ok := g[to]; !ok {
		g.AddVertex(to)
	}
	g[from][to] = e
}

func (g Graph) AddUndirected(from, to Vertex, e Edge) {
	g.AddDirected(from, to, e)
	g.AddDirected(to, from, e)
}

func (g Graph) RemoveVertex(v Vertex) {
	delete(g, v)
	for from := range g {
		g.RemoveDirected(from, v)
	}
}

func (g Graph) RemoveDirected(from, to Vertex) {
	if es, ok := g[from]; ok {
		delete(es, to)
	}
}

func (g Graph) RemoveUndirected(from, to Vertex) {
	g.RemoveDirected(from, to)
	g.RemoveDirected(to, from)
}

func (g Graph) Vertices() (vs []Vertex) {
	for v := range g {
		vs = append(vs, v)
	}
	return
}

func (g Graph) Edges(v Vertex) map[Vertex]Edge {
	return g[v]
}

func (g Graph) Edge(from, to Vertex) Edge {
	es := g.Edges(from)
	if es == nil {
		return nil
	}
	return es[to]
}
