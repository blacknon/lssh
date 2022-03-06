package sftp

func DupPermutationsRecursive0(n, k int) [][]int {
	if k == 0 {
		pattern := []int{}
		return [][]int{pattern}
	}

	ans := [][]int{}
	for num := 0; num < n; num++ {
		childPatterns := DupPermutationsRecursive0(n, k-1)
		for _, childPattern := range childPatterns {
			pattern := append([]int{num}, childPattern...)
			ans = append(ans, pattern)
		}
	}

	return ans
}
