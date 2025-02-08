package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const bufferDrainInterval = 5 * time.Second
const bufferSize = 10

type RingBuffer struct {
	data []int
	pos  int
	size int
	mu   sync.Mutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{data: make([]int, size), size: size, pos: -1}
}

func (rb *RingBuffer) Push(value int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.pos == rb.size-1 {
		copy(rb.data, rb.data[1:])
		rb.data[rb.pos] = value
	} else {
		rb.pos++
		rb.data[rb.pos] = value
	}
}

func (rb *RingBuffer) Get() []int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.pos < 0 {
		return nil
	}
	data := rb.data[:rb.pos+1]
	rb.pos = -1
	return data
}

func filterNegatives(done <-chan bool, input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for {
			select {
			case num := <-input:
				if num >= 0 {
					output <- num
				}
			case <-done:
				return
			}
		}
	}()
	return output
}

func filterNonMultiplesOfThree(done <-chan bool, input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for {
			select {
			case num := <-input:
				if num != 0 && num%3 == 0 {
					output <- num
				}
			case <-done:
				return
			}
		}
	}()
	return output
}

func bufferStage(done <-chan bool, input <-chan int) <-chan int {
	output := make(chan int)
	buffer := NewRingBuffer(bufferSize)

	go func() {
		defer close(output)
		for {
			select {
			case num := <-input:
				buffer.Push(num)
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.After(bufferDrainInterval):
				bufferedData := buffer.Get()
				if bufferedData != nil {
					for _, num := range bufferedData {
						output <- num
					}
				}
			case <-done:
				return
			}
		}
	}()

	return output
}

func dataSource() (<-chan int, <-chan bool, chan struct{}) {
	input := make(chan int)
	done := make(chan bool)
	nextPrompt := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(os.Stdin)
		for {
			<-nextPrompt
			scanner.Scan()
			text := strings.TrimSpace(scanner.Text())
			if strings.EqualFold(text, "exit") {
				return
			}
			num, err := strconv.Atoi(text)
			if err != nil {
				nextPrompt <- struct{}{}
				continue
			}
			input <- num
		}
	}()
	nextPrompt <- struct{}{}
	return input, done, nextPrompt
}

func consumer(done <-chan bool, input <-chan int, nextPrompt chan struct{}) {
	for {
		select {
		case <-input:
			nextPrompt <- struct{}{}
		case <-done:
			return
		}
	}
}

func main() {
	source, done, nextPrompt := dataSource()

	stage1 := filterNegatives(done, source)
	stage2 := filterNonMultiplesOfThree(done, stage1)
	stage3 := bufferStage(done, stage2)

	consumer(done, stage3, nextPrompt)
}
