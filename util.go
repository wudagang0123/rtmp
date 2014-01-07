package rtmp

import (
    "io"
)

func readBuffer(r io.Reader,n int32) ([]byte,error) {
    b := make([]byte,n)
    _,err := r.Read(b)
    return b,err
}

func writeBuffer(w io.Writer,buf []byte) error {
    _,err := w.Write(buf)
    return err
}