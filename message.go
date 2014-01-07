package rtmp 

import (
    "fmt"
    "io"
    "net"
    "bytes"
    "encoding/binary"
)

const (
    RTMP_STREAM_PUBLISHER = 0
    RTMP_STREAM_PLAYER = 1
)

const (
    RTMP_STREAM_INIT = 0
    RTMP_STREAM_CONNECTED = 1
    RTMP_STREAM_ACTIVE = 2
)

const (
    RTMP_SIG_SIZE = 1536
    AMF_PACAKGE_SIZE = 128
)

const (
    MSG_CHUNK_SIZE         = 1
    MSG_ABORT              = 2
    MSG_ACK                = 3
    MSG_USER               = 4
    MSG_ACK_SIZE           = 5
    MSG_BANDWIDTH          = 6
    MSG_EDGE               = 7
    MSG_AUDIO              = 8
    MSG_VIDEO              = 9
    MSG_AMF3_META          = 15
    MSG_AMF3_SHARED        = 16
    MSG_AMF3_CMD           = 17
    MSG_AMF_META           = 18
    MSG_AMF_SHARED         = 19
    MSG_AMF_CMD            = 20
    MSG_AGGREGATE          = 22
    MSG_MAX                = 22
)

type chunkHeader struct {
    cfmt byte
    csid int32
    timestamp int32
    mlen int32
    typeid byte
    streamId int32
}

type rtmpPacket struct {
    chunkHeader
    data *bytes.Buffer

    curts int32
}

type rtmpStream struct {
    app string
    role int32
    ts int32
    state int32
    msgs map[int32]*rtmpPacket
    conn net.Conn
}

func (this *rtmpStream) readChunkHeader() *chunkHeader {
    h := &chunkHeader{}

    fmt.Println("=====readChunkHeader=====")
    buf := amf_decode_byte(this.conn)
    fmt.Println("rtmp basic header: ",buf)
    
    h.cfmt = (buf & 0xC0) >> 6
    fmt.Println("header format: ",h.cfmt)

    h.csid = int32(buf & 0x3F)
    if h.csid == 0x00 {
        j := amf_decode_byte(this.conn)
        h.csid = int32(j) + 64
    }

    if h.csid == 0x3F {
        j := amf_decode_int16(this.conn)
        h.csid = int32(j) + 64
    }

    if h.cfmt == 0 {
        h.timestamp = amf_decode_int24(this.conn)
        h.mlen = amf_decode_int24(this.conn)
        h.typeid = amf_decode_byte(this.conn)
        h.streamId = amf_decode_int32LE(this.conn)
    }
    
    if h.cfmt == 1 {
        h.timestamp = amf_decode_int24(this.conn)
        h.mlen = amf_decode_int24(this.conn)
        h.typeid = amf_decode_byte(this.conn)
    }

    if h.cfmt == 2 {
        h.timestamp = amf_decode_int24(this.conn)
    }
 
    if h.timestamp == 0xFFFFFF {
        h.timestamp = amf_decode_int32(this.conn)
    }

    fmt.Println("chunk stream id: ",h.csid)
    fmt.Println("timestamp: ",h.timestamp)
    fmt.Println("amfSize: ",h.mlen)
    fmt.Println("amfType: ",h.typeid)
    fmt.Println("streamID:",h.streamId)
    fmt.Println("=====readChunkHeader===end=====")
    return h
}

func (this *rtmpStream) readChunkPacket() *rtmpPacket {
    fmt.Println("===========readChunkPacket============")

    var size int32
    h := this.readChunkHeader()
    packet,ok := this.msgs[h.csid]
    if !ok {
        packet = &rtmpPacket{data:&bytes.Buffer{}}
        this.msgs[h.csid] = packet
    }
    
    switch h.cfmt {
    case 0:
        packet.csid = h.csid
        packet.timestamp = h.timestamp
        packet.mlen = h.mlen
        packet.typeid = h.typeid
        packet.streamId = h.streamId
        packet.curts = packet.timestamp
    case 1:
        packet.timestamp = h.timestamp
        packet.mlen = h.mlen
        packet.typeid = h.typeid
        packet.curts += packet.timestamp
    case 2:
        packet.timestamp = h.timestamp
        packet.curts += packet.timestamp
    }

    left := packet.mlen - int32(packet.data.Len())
    size = 128
    if size >= left {
        size = left
    }

    if size > 0 {
        io.CopyN(packet.data,this.conn,int64(size))
    }

    if size == left {
        m := new(rtmpPacket)
        *m = *packet
        packet.data = &bytes.Buffer{}
        //fmt.Println(packet.data.Bytes())
        fmt.Println("===========readChunkPacket=====end=======")
        return m
    }

    return nil
}

func (this *rtmpStream) writeChunkPacket(packet *rtmpPacket) {
    buf := new(bytes.Buffer)

    headerType := (packet.cfmt << 6) & 0xC0
    headerType = headerType | (byte(packet.csid) & 0x3F)

    fmt.Println("packet header type: ",packet.cfmt)
    fmt.Println("header type: ",headerType)
    switch packet.cfmt {
    case 0:
        binary.Write(buf,binary.BigEndian,headerType)
        binary.Write(buf,binary.BigEndian,amf_encode_int24(packet.timestamp))
        binary.Write(buf,binary.BigEndian,amf_encode_int24(int32(packet.data.Len())))
        binary.Write(buf,binary.BigEndian,packet.typeid)
        binary.Write(buf,binary.BigEndian,amf_encode_int32LE(packet.streamId))
    case 1:
        binary.Write(buf,binary.BigEndian,headerType)
        binary.Write(buf,binary.BigEndian,amf_encode_int24(packet.timestamp))
        binary.Write(buf,binary.BigEndian,amf_encode_int24(int32(packet.data.Len())))
        binary.Write(buf,binary.BigEndian,packet.typeid)
    case 2:
        binary.Write(buf,binary.BigEndian,headerType)
        binary.Write(buf,binary.BigEndian,amf_encode_int24(packet.timestamp))
    }

    size := 128
    left := packet.data.Len()

    for left > 0 {
        if size > left {
            size = left
        }

        io.CopyN(buf,packet.data,int64(size))
        left -= size
        if left == 0 {
            break
        }

        byteHeader := make([]byte,1)
        byteHeader[0] = (0x3 << 6) | byte(packet.csid)
        buf.Write(byteHeader)
    }

    //fmt.Println(buf.Bytes())
    this.conn.Write(buf.Bytes())
}

func (this *rtmpStream) streamBegin(csid int32,streamId int32,data []byte) {
    packet := &rtmpPacket{}
    packet.cfmt = 0
    packet.csid = csid
    packet.typeid = MSG_USER
    packet.timestamp = 0
    packet.streamId = streamId
    packet.data = bytes.NewBuffer(data)

    this.writeChunkPacket(packet)
}

func (this *rtmpStream) streamEnd(csid int32,streamId int32,data []byte) {
    packet := &rtmpPacket{}
    packet.cfmt = 0
    packet.csid = csid
    packet.typeid = MSG_USER
    packet.timestamp = 0
    packet.streamId = streamId
    packet.data = bytes.NewBuffer(data)

    this.writeChunkPacket(packet)
}

func (this *rtmpStream) writeVideoPacket(cfmt byte,csid int32,streamId int32,data *rtmpPacket) {
    packet := &rtmpPacket{}
    packet.cfmt = cfmt
    packet.csid = csid
    packet.typeid = MSG_VIDEO
    packet.timestamp = data.timestamp
    packet.streamId = streamId
    packet.data = bytes.NewBuffer(data.data.Bytes())
    
    this.writeChunkPacket(packet)
}

func (this *rtmpStream) writeAudioPacket(cfmt byte,csid int32,streamId int32,data *rtmpPacket) {
    packet := &rtmpPacket{}
    packet.cfmt = cfmt
    packet.csid = csid
    packet.typeid = MSG_AUDIO
    packet.timestamp = data.timestamp
    packet.streamId = streamId
    packet.data = bytes.NewBuffer(data.data.Bytes())
    
    this.writeChunkPacket(packet)
}

func (this *rtmpStream) Close() {
    this.conn.Close()
}