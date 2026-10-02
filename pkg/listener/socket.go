package listener

// RawSocket is a platform-neutral description of a listening TCP socket,
// produced before process enrichment. Platform-specific socket sources
// (Darwin: lsof; Linux: /proc via gopsutil) return these. Keeping this type
// free of OS details is what lets Scanner stay portable.
type RawSocket struct {
	PID     int32
	Port    uint32
	Address string
	Proto   string
}
