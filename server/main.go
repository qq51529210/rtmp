package main

func main() {
	s := new(Server)
	s.Address = "127.0.0.1:1935"
	s.Version = 100
	s.Listen()
}
