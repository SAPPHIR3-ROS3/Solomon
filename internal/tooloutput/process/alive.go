package process

func Alive(pid int) bool {
	return processAlive(pid)
}
