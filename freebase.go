package main

import (
	"io"
	"bufio"
	"compress/bzip2"
	"os"
	"strings"
	"time"
	"log"
	"sync"
)

type quad struct {
	left, prop, right, data string
}

func quads(r *bufio.Reader) <-chan quad {
	quads := make(chan quad)

	go func(result chan<- quad) {
		defer close(result)

		err := error(nil)
		for err == nil {
			left, err := r.ReadString('\t')
			if err != nil {
				return
			}

			prop, err := r.ReadString('\t')
			if err != nil {
				return
			}

			right, err := r.ReadString('\t')
			if err != nil {
				return
			}

			data, err := r.ReadString('\n')
			if err != nil {
				return
			}

			// sliceing to remove trailing whitespace. This is unicode safe, since we split on bytes.
			result <- quad{left[:len(left)-1], prop[:len(prop)-1], right[:len(right)-1], data[:len(data)-1]}
		}

		close(result)
	}(quads)

	return quads
}

type fileProgressReader struct {
	f *os.File
	progress, total int64

	reportLock sync.Mutex
	lastReport int
}

func newProgressReader(path string) (*fileProgressReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fp := &fileProgressReader{f: f}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fp.total = stat.Size()

	return fp, nil
}

func (f fileProgressReader) Write(b []byte) (int, error) {
	return f.f.Write(b)
}

func (f *fileProgressReader) Read(b []byte) (int, error) {
	n, err := f.f.Read(b)
	f.progress += int64(n)

	if now := time.Now().Second(); now > f.lastReport + 5 {
		log.Printf("Progress: %.2f%% (%d / %d bytes)", float32(f.progress) / float32(f.total), f.progress, f.total)
		f.lastReport = now
	}

	return n, err
}

func (graph *Graph) ReadFreebase(path string) error {
	f, err := newProgressReader(path)
	if err != nil {
		return err
	}

	r := io.Reader(f)
	if strings.HasSuffix(path, ".bz2") {
		r = bzip2.NewReader(r)
	}

	for quad := range quads(bufio.NewReader(r)) {
		left, err := graph.Add(Node{Value: quad.left})
		if err != nil {
			return err
		}

		prop, err := graph.Add(Node{Value: quad.prop})
		if err != nil {
			return err
		}

		right, err := graph.Add(Node{Value: quad.right})
		if err != nil {
			return err
		}

		_, err = graph.Add(Node{Value: quad.data, Edge: &Edge{Left: left, Prop: prop, Right: right}})
		if err != nil {
			return err
		}
	}

	return nil
}