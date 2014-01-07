package rtmp

import (
    "fmt"
    "io"
    "bytes"
    "encoding/binary"
)

const (
    AMF_NUMBER             = 0x00
    AMF_BOOLEAN            = 0x01
    AMF_STRING             = 0x02
    AMF_OBJECT             = 0x03
    AMF_NULL               = 0x05
    AMF_ARRAY_NULL         = 0x06
    AMF_MAP                = 0x08
    AMF_END                = 0x09
    AMF_ARRAY              = 0x0a
)

type amfObj struct {
    objType byte
    str string
    i byte
    buf []byte
    objs map[string]*amfObj
    f64 float64
}

func amf_encode(obj *amfObj) []byte {
    val := new(bytes.Buffer)

    binary.Write(val,binary.BigEndian,obj.objType)

    switch obj.objType {
    case AMF_NUMBER:
        binary.Write(val,binary.BigEndian,obj.f64)
    case AMF_BOOLEAN:
        binary.Write(val,binary.BigEndian,obj.i)
    case AMF_STRING:
        str := amf_encode_string(obj.str)
        binary.Write(val,binary.BigEndian,str)
    case AMF_OBJECT:
        o := amf_encode_core_object(obj)
        binary.Write(val,binary.BigEndian,o)
    }

    return val.Bytes()
}

func amf_decode(r io.Reader) *amfObj {
    obj := &amfObj{}
    objType := amf_decode_byte(r)
    fmt.Println("objType: ",objType)

    switch objType {
    case AMF_NUMBER:
        obj.f64 = amf_decode_int64(r)
        //fmt.Println("corenumber: ",obj.f64)
    case AMF_BOOLEAN:
        obj.i = amf_decode_byte(r)
        //fmt.Println("coreboolean: ",obj.i)
    case AMF_STRING:
        n := amf_decode_int16(r)
        obj.str = amf_decode_string(r,int32(n))
        //fmt.Println("corestring: ",obj.str)
    case AMF_OBJECT:
        obj.objs = amf_decode_core_object(r)
    case AMF_NULL:
        fmt.Println("null")
    case AMF_ARRAY_NULL:
        fmt.Println("undefined")
    case AMF_MAP:
        amf_decode_core_map(r)
    }

    return obj
}

func amf_encode_core_object(obj *amfObj) []byte {
    val := new(bytes.Buffer)

    for k,v := range(obj.objs) {
        val.Write(amf_encode_string(k))

        binary.Write(val,binary.BigEndian,v.objType)
        switch v.objType {
        case AMF_NUMBER:
            binary.Write(val,binary.BigEndian,v.f64)
        case AMF_BOOLEAN:
            binary.Write(val,binary.BigEndian,v.i)
        case AMF_STRING:
            val.Write(amf_encode_string(v.str))
        case AMF_OBJECT:
            o := amf_encode_core_object(v)
            binary.Write(val,binary.BigEndian,o)
        }
    }

    end := amf_encode_int24(9)
    binary.Write(val,binary.BigEndian,end)

    return val.Bytes()
}

func amf_decode_core_object(r io.Reader) map[string]*amfObj {
    fmt.Println("decode obj")

    objMap := make(map[string]*amfObj)

    for {
        n := amf_decode_int16(r)
        if n == 0 {
            break
        }

        key := amf_decode_string(r,int32(n))
        //fmt.Println("key: ",key)

        objType := amf_decode_byte(r)
        //fmt.Println("objType1: ",objType)

        obj := &amfObj{}
        switch objType {
        case AMF_NUMBER:
            obj.f64 = amf_decode_int64(r)
            //fmt.Println("corenumber: ",obj.f64)
        case AMF_BOOLEAN:
            obj.i = amf_decode_byte(r)
            //fmt.Println("coreboolean: ",obj.i)
        case AMF_STRING:
            n = amf_decode_int16(r)
            obj.str = amf_decode_string(r,int32(n))
            //fmt.Println("corestring: ",obj.str)
        case AMF_OBJECT:
            obj.objs = amf_decode_core_object(r)
        case AMF_NULL:
            //fmt.Println("null")
        case AMF_MAP:
            amf_decode_core_map(r)
        }

        objMap[key] = obj
    }

    return objMap
}

func amf_decode_core_map(r io.Reader) {
    fmt.Println("decode core map")
    num := amf_decode_int24(r)
    fmt.Println("map num: ",num)
    _ = amf_decode_core_object(r)
}

func amf_encode_string(str string) []byte {
    strlen := int16(len(str))
    val := new(bytes.Buffer)
    binary.Write(val,binary.BigEndian,strlen)
    val.Write([]byte(str))
    return val.Bytes()
}

func amf_decode_string(r io.Reader,n int32) string {
    data,err := readBuffer(r,n)
    if err != nil {
        panic(err)
    }

    return string(data)
}

func amf_decode_byte(r io.Reader) byte{
    data,err := readBuffer(r,1)
    if err != nil {
        panic(err)
    }

    return data[0]  
}

func amf_encode_int16(data int16) []byte {
    val := new(bytes.Buffer)
    binary.Write(val,binary.BigEndian,data)
    return val.Bytes()
}

func amf_decode_int16(r io.Reader) int16 {
    var val int16
    
    data,err := readBuffer(r,2)
    if err != nil {
        panic(err)
    }

    buf := bytes.NewBuffer(data)
    binary.Read(buf,binary.BigEndian,&val)

    return val
}

func amf_encode_int24(data int32) []byte {
    val := make([]byte,3)
    val[0] = byte((data & 0x00FF0000) >> 16)
    val[1] = byte((data & 0x0000FF00) >> 8)
    val[2] = byte(data & 0x000000FF)
    return val
}

func amf_decode_int24(r io.Reader) int32 {
    data,err := readBuffer(r,3)
    if err != nil {
        panic(err)
    }

    val := (int32(data[0]) << 16) | (int32(data[1]) << 8) | int32(data[2]) 

    return val 
}

func amf_encode_int32(data int32) []byte {
    val := new(bytes.Buffer)
    binary.Write(val,binary.BigEndian,data)
    return val.Bytes()
}

func amf_decode_int32(r io.Reader) int32 {
    var val int32
    
    data,err := readBuffer(r,4)
    if err != nil {
        panic(err)
    }

    buf := bytes.NewBuffer(data)
    binary.Read(buf,binary.BigEndian,&val)

    return val 
}

func amf_encode_int32LE(data int32) []byte {
    val := new(bytes.Buffer)
    binary.Write(val,binary.LittleEndian,data)
    return val.Bytes()
}

func amf_decode_int32LE(r io.Reader) int32 {
    var val int32
    
    data,err := readBuffer(r,4)
    if err != nil {
        panic(err)
    }

    buf := bytes.NewBuffer(data)
    binary.Read(buf,binary.LittleEndian,&val)

    return val 
}

func amf_encode_int64(data float64) []byte {
    val := new(bytes.Buffer)
    binary.Write(val,binary.BigEndian,data)
    return val.Bytes()
}

func amf_decode_int64(r io.Reader) float64 {
    var val float64

    data,err := readBuffer(r,8)
    if err != nil {
        panic(err)
    }

    buf := bytes.NewBuffer(data)
    binary.Read(buf,binary.BigEndian,&val)

    return val 
}