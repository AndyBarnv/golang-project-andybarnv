package main

import "fmt"

func twoSum(nums []int, target int) []int {
	var res []int
	for i := 0; i < len(nums)-1; i++ {
		for j := i + 1; j < len(nums); j++ {
			a, b := nums[i], nums[j]
			if a+b == target {
				res = append(res, i, j)
			}
		}

	}
	return res
}

func main() {
	test_slice := []int{2, 3, 7, 4, 8}
	res := twoSum(test_slice, 9)
	fmt.Println(res)
}
