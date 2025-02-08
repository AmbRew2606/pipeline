package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Настройки кольцевого буфера
const bufferDrainInterval = 5 * time.Second // Интервал сброса буфера
const bufferSize = 10                       // Размер буфера

// RingBuffer - структура кольцевого буфера
type RingBuffer struct {
	data []int
	pos  int
	size int
	mu   sync.Mutex
}

// NewRingBuffer - создаёт новый кольцевой буфер
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{data: make([]int, size), size: size, pos: -1}
}

// Push - добавляет элемент в буфер
func (rb *RingBuffer) Push(value int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.pos == rb.size-1 {
		// Буфер заполнен, сдвиг элементов
		copy(rb.data, rb.data[1:])
		rb.data[rb.pos] = value
	} else {
		rb.pos++
		rb.data[rb.pos] = value
	}
}

// Get - извлекает все элементы из буфера и очищает его
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

// Функция фильтрации отрицательных чисел
func filterNegatives(done <-chan bool, input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for {
			select {
			case num := <-input:
				fmt.Printf(">>> Этап 1: Фильтрация отрицательных чисел. Входные данные: %d\n", num)
				if num >= 0 {
					output <- num
					fmt.Printf(">>> Этап 1: Число %d прошло фильтрацию\n", num)
				} else {
					fmt.Printf(">>> Этап 1: Число %d отфильтровано (отрицательное)\n", num)
				}
			case <-done:
				return
			}
		}
	}()
	return output
}

// Функция фильтрации чисел, не кратных 3 (исключая 0)
func filterNonMultiplesOfThree(done <-chan bool, input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for {
			select {
			case num := <-input:
				fmt.Printf(">>> Этап 2: Фильтрация чисел, не кратных 3. Входные данные: %d\n", num)
				if num != 0 && num%3 == 0 {
					output <- num
					fmt.Printf(">>> Этап 2: Число %d прошло фильтрацию\n", num)
				} else {
					fmt.Printf(">>> Этап 2: Число %d отфильтровано (не кратно 3)\n", num)
				}
			case <-done:
				return
			}
		}
	}()
	return output
}

// Функция буферизации данных
func bufferStage(done <-chan bool, input <-chan int) <-chan int {
	output := make(chan int)
	buffer := NewRingBuffer(bufferSize)

	go func() {
		defer close(output)
		for {
			select {
			case num := <-input:
				fmt.Printf(">>> Этап 3: Буферизация данных. Число добавлено в буфер: %d\n", num)
				buffer.Push(num)
			case <-done:
				return
			}
		}
	}()

	// Горутина сброса буфера через интервал
	go func() {
		for {
			select {
			case <-time.After(bufferDrainInterval):
				bufferedData := buffer.Get()
				if bufferedData != nil {
					fmt.Printf(">>> Этап 3: Сброс буфера. Данные: %v\n", bufferedData)
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

// Источник данных из консоли
func dataSource() (<-chan int, <-chan bool, chan struct{}) {
	input := make(chan int)
	done := make(chan bool)
	nextPrompt := make(chan struct{}) // Канал для управления выводом подсказки
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(os.Stdin)
		for {
			<-nextPrompt
			fmt.Println("Введите число: ")
			fmt.Println("Или 'exit' для выхода")
			scanner.Scan()
			text := strings.TrimSpace(scanner.Text())
			if strings.EqualFold(text, "exit") {
				fmt.Println("Завершение программы...")
				return
			}
			num, err := strconv.Atoi(text)
			if err != nil {
				fmt.Println("Ошибка: введено не число!")
				nextPrompt <- struct{}{} // Сигналить снова
				continue
			}
			fmt.Printf(">>> Число введено: %d\n", num)
			input <- num
		}
	}()
	nextPrompt <- struct{}{}
	return input, done, nextPrompt
}

func consumer(done <-chan bool, input <-chan int, nextPrompt chan struct{}) {
	for {
		select {
		case num := <-input:
			fmt.Printf(">>> Финальный этап: Обработанные данные: %d\n", num)
			nextPrompt <- struct{}{} // Сигнал для вывода следующей подсказки
		case <-done:
			return
		}
	}
}

// Основная функция
func main() {
	source, done, nextPrompt := dataSource()

	// Пайплайн обработки
	stage1 := filterNegatives(done, source)
	stage2 := filterNonMultiplesOfThree(done, stage1)
	stage3 := bufferStage(done, stage2)

	consumer(done, stage3, nextPrompt)
}
