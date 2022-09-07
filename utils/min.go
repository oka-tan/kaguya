//Package utils provides some basic utils for usage within
//Kaguya
package utils

//Min calculates the min between two ints
//Golang's math.Min only accepts float arguments (
//as of writing)
func Min(a int, b int) int {
	if a < b {
		return a
	}

	return b
}
