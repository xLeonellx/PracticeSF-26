package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var bufferClearInterval int

const bufferSize int = 10

type RingBuffer struct {
	array []int
	pos   int
	size  int
	m     sync.Mutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{make([]int, size), -1, size, sync.Mutex{}}
}

func (r *RingBuffer) Push(el int) {
	r.m.Lock()
	defer r.m.Unlock()
	if r.pos == r.size-1 {
		for i := 1; i <= r.size; i++ {
			r.array[i-1] = r.array[i]
		}
		r.array[r.pos] = elem
	} else {
		r.pos++
		r.array[r.pos] = el
	}
}

func (r *RingBuffer) Get() []int {
	if r.pos < 0 {
		return nil
	}
	r.m.Lock()
	defer r.m.Unlock()
	output := r.array[:r.pos+1]
	r.pos = -1
	return output
}

type ConvInt func(<-chan bool, <-chan int) <-chan int

type PipelineInt struct {
	stages []ConvInt
	done   <-chan bool
}

func NewPipelineInt(done <-chan bool, stages ...ConvInt) *PipelineInt {
	return &PipelineInt{done: done, stages: stages}
}

func (p *PipelineInt) Run(source <-chan int) <-chan int {
	var c <-chan int = source
	for index := range p.stages {
		c = p.runConvInt(p.stages[index], c)
	}
	return c
}

func (p *PipelineInt) runConvInt(stage ConvInt, sourceChan <-chan int) <-chan int {
	return stage(p.done, sourceChan)
}

func main() {

	l := log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)

	dataSource := func() (<-chan int, <-chan bool) {
		c := make(chan int)
		done := make(chan bool)
		go func() {
			defer close(done)
			scanner := bufio.NewScanner(os.Stdin)
			var data string
			for {
				scanner.Scan()
				data = scanner.Text()
				if strings.EqualFold(data, "exit") {
					l.Println("Программа завершила работу! До свидания!")
					return
				}
				i, err := strconv.Atoi(data)
				if err != nil {
					l.Println("Какая досада! Программа обрабатывает только целые числа!")
					continue
				}
				c <- i
			}
		}()
		return c, done
	}

	negativeFilterConvInt := func(done <-chan bool, c <-chan int) <-chan int {
		convertedIntChan := make(chan int)
		go func() {
			for {
				select {
				case data := <-c:
					if data > 0 {
						select {
						case convertedIntChan <- data:
						case <-done:
							return
						}
					}
				case <-done:
					return
				}
			}
		}()
		return convertedIntChan
	}

	specialFilterConvInt := func(done <-chan bool, c <-chan int) <-chan int {
		filteredIntChan := make(chan int)
		go func() {
			for {
				select {
				case data := <-c:
					if data != 0 && data%3 == 0 {
						select {
						case filteredIntChan <- data:
						case <-done:
							return
						}
					}
				case <-done:
					return
				}
			}
		}()
		return filteredIntChan
	}

	bufferConvInt := func(done <-chan bool, c <-chan int) <-chan int {
		bufferedIntChan := make(chan int)
		buffer := NewRingBuffer(bufferSize)
		go func() {
			for {
				select {
				case data := <-c:
					buffer.Push(data)
				case <-done:
					return
				}
			}
		}()

		go func() {
			for {
				select {
				case <-time.After(time.Duration(bufferClearInterval) * 1000000000):
					bufferData := buffer.Get()
					if bufferData != nil {
						for _, data := range bufferData {
							select {
							case bufferedIntChan <- data:
							case <-done:
								return
							}
						}
					}
				case <-done:
					return
				}
			}
		}()
		return bufferedIntChan
	}

	consumer := func(done <-chan bool, c <-chan int) {
		for {
			select {
			case data := <-c:
				l.Printf("Обработаны данные: %d\n", data)
			case <-done:
				return
			}
		}
	}

	l.Println("Добро пожаловать в программу!")
	l.Println("Пожалуйста введите интервал очистки! (в секундах)")
	fmt.Scanln(&bufferClearInterval)
	l.Println("Пожалуйста введите ваши данные!")
	source, done := dataSource()
	pipeline := NewPipelineInt(done, negativeFilterConvInt, specialFilterConvInt, bufferConvInt)
	consumer(done, pipeline.Run(source))
}
