package utils

import (
	"encoding/json"
	"sort"
)

// ArrayList 定义泛型集合结构体
type ArrayList[T any] struct {
	elements []T
}

// GetElements 获取底层数组
func (a *ArrayList[T]) GetElements() []T {
	return a.elements
}

// NewArrayList 创建一个新的 ArrayList 实例
func NewArrayList[T any]() *ArrayList[T] {
	return &ArrayList[T]{
		elements: make([]T, 0),
	}
}

// NewArrayListElements 创建一个新的 ArrayList 实例
func NewArrayListElements[T any](elements []T) *ArrayList[T] {
	return &ArrayList[T]{
		elements: elements,
	}
}

// Add 向 ArrayList 中添加元素 引用共享
func (a *ArrayList[T]) Add(element T) {
	a.elements = append(a.elements, element)
}

// AddAll 向 ArrayList 中添加多个元素 引用不共享
func (a *ArrayList[T]) AddAll(elements []T) {
	newLen := len(a.elements) + len(elements)
	if cap(a.elements) < newLen {
		newSlice := make([]T, newLen)
		copy(newSlice, a.elements)
		a.elements = newSlice
	}
	a.elements = a.elements[:newLen]
	copy(a.elements[len(a.elements)-len(elements):], elements)
}

// Get 根据索引获取元素
func (a *ArrayList[T]) Get(index int) T {
	if index < 0 || index >= len(a.elements) {
		var zero T
		return zero
	}
	return a.elements[index]
}

// Remove 根据索引删除元素
func (a *ArrayList[T]) Remove(index int) bool {
	if index < 0 || index >= len(a.elements) {
		return false
	}
	copy(a.elements[index:], a.elements[index+1:])
	a.elements = a.elements[:len(a.elements)-1]
	return true
}

// Contain 检查集合是否包含某个元素
//func (a *ArrayList[T]) Contain(element T) bool {
//	for _, e := range a.elements {
//		marshal, _ := json.Marshal(element)
//		marshal2, _ := json.Marshal(e)
//		if string(marshal) == string(marshal2) {
//			return true
//		}
//	}
//	return false
//}

// Size 返回集合长度
func (a *ArrayList[T]) Size() int {
	return len(a.elements)
}

// FindIndexBy FindIndexOf 查找满足条件的元素的索引
func (a *ArrayList[T]) FindIndexBy(predicate func(T) bool) int {
	for i, element := range a.elements {
		if predicate(element) {
			return i
		}
	}
	return -1
}

// RemoveBy 移除满足条件的元素
func (a *ArrayList[T]) RemoveBy(predicate func(T) bool) {
	i := 0
	for i < len(a.elements) {
		if predicate(a.elements[i]) {
			a.Remove(i)
		} else {
			i++
		}
	}
}

// Concat 连接一个相同泛型的新数组，并返回当前实例
func (a *ArrayList[T]) Concat(other *ArrayList[T]) *ArrayList[T] {
	a.elements = append(a.elements, other.elements...)
	return a
}

// Some 遍历元素，只要有一个元素满足条件就返回 true
func (a *ArrayList[T]) Some(predicate func(T) bool) bool {
	for _, element := range a.elements {
		if predicate(element) {
			return true
		}
	}
	return false
}

// ForEach 遍历元素
func (a *ArrayList[T]) ForEach(predicate func(index int, item T) bool) {
	for i, element := range a.elements {
		b := predicate(i, element)
		if b == false {
			break
		}

	}
}

// Every 遍历元素，如果所有元素都满足条件则返回 true
func (a *ArrayList[T]) Every(predicate func(T) bool) bool {
	for _, element := range a.elements {
		if !predicate(element) {
			return false
		}
	}
	return true
}

// Partition 根据元素总数将集合拆分成多个子集合
func (a *ArrayList[T]) Partition(subListSize int) []ArrayList[T] {
	if subListSize <= 0 {
		return nil
	}
	var result []ArrayList[T]
	for i := 0; i < a.Size(); i += subListSize {
		subList := NewArrayList[T]()
		end := i + subListSize
		if end > a.Size() {
			end = a.Size()
		}
		for j := i; j < end; j++ {
			subList.Add(a.Get(j))
		}
		result = append(result, *subList)
	}
	return result
}

// Filter 过滤满足条件的元素，返回一个新实例
func (a *ArrayList[T]) Filter(predicate func(T) bool) *ArrayList[T] {
	filteredList := NewArrayList[T]()
	for _, element := range a.elements {
		if predicate(element) {
			filteredList.Add(element)
		}
	}
	return filteredList
}

// Find 过滤满足条件的元素，返回一个新实例
func (a *ArrayList[T]) Find(predicate func(T) bool) T {
	for _, element := range a.elements {
		if predicate(element) {
			return element
		}
	}
	var wt T
	return wt
}

// Clear 清空数组
func (a *ArrayList[T]) Clear() {
	a.elements = a.elements[:0]
}

// ToJsonStr 将集合转换为 JSON 字符串
func (a *ArrayList[T]) ToJsonStr() (string, error) {
	bytes, err := json.Marshal(a.elements)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// FromJsonStr 将 JSON 字符串转换为 ArrayList 实例
func (a *ArrayList[T]) FromJsonStr(jsonStr string) (*ArrayList[T], error) {
	var newElements []T
	err := json.Unmarshal([]byte(jsonStr), &newElements)
	if err != nil {
		return nil, err
	}
	a.elements = newElements
	return a, nil
}

// IsNotEmpty 不为空
func (a *ArrayList[T]) IsNotEmpty() bool {
	return len(a.elements) > 0
}

// IsEmpty 为空
func (a *ArrayList[T]) IsEmpty() bool {
	return len(a.elements) == 0
}

// Sort 排序
func (a *ArrayList[T]) Sort(less func(i, j int) bool) {
	sort.Slice(a.elements, less)
}
