package rtmp 

import (
    "fmt"
    "time"
    "runtime"
    "net"
    "bytes"
    "math/rand"
    "encoding/binary"
)

var (
    appMap map[string] chan *rtmpPacket = make(map[string] chan *rtmpPacket)
)

func handShanke(conn net.Conn) int {
    //recv c0
    c0 := make([]byte,1)
    size,err := conn.Read(c0)
    if err != nil {
        fmt.Println("Read c0 error")
        return -1
    }

    //recv c1
    c1 := make([]byte,1536)
    size,err = conn.Read(c1)
    if err != nil {
        fmt.Println("Read c1 error")
        return -1
    }

    if(size <= 0){
        return -1
    }
    //fmt.Println("=====================c1=======================")
    //fmt.Println(c1)

    //get s1 timestamp and rand 
    c1_buf := bytes.NewBuffer(c1)
    var c1_stamp int32
    binary.Read(c1_buf,binary.BigEndian,&c1_stamp)
    var c1_zero int32
    binary.Read(c1_buf,binary.BigEndian,&c1_zero)
    c1_rand := make([]byte,1528)
    binary.Read(c1_buf,binary.BigEndian,c1_rand)

    //send s0
    conn.Write(c0)

    //send s1
    s1_stamp := int32(time.Now().Unix())
    s1_buf := new(bytes.Buffer)
    binary.Write(s1_buf,binary.BigEndian,s1_stamp)
    s1_zero := int32(0)
    binary.Write(s1_buf,binary.BigEndian,s1_zero)

    rand.Seed(time.Now().Unix())
    for i:=0; i<1528/4; i++ {
        binary.Write(s1_buf,binary.BigEndian,rand.Int31())
    }

    //fmt.Println("=====================s1=======================")
    //fmt.Println(s1_buf.Bytes())
    conn.Write(s1_buf.Bytes())

    //recv c2
    c2 := make([]byte,1536)
    size,err = conn.Read(c2)
    if err != nil {
        fmt.Println("read handshake c2 error",err)
        return -1
    }

    if(size <= 0){
        return -1
    }

    //fmt.Println("=====================c2=======================")
    //fmt.Println(c2)

    //send s2
    s2_buf := new(bytes.Buffer)
    binary.Write(s2_buf,binary.BigEndian,c1_stamp)
    binary.Write(s2_buf,binary.BigEndian,s1_zero)
    binary.Write(s2_buf,binary.BigEndian,c1_rand)

    //fmt.Println("=====================s2=======================")
    //fmt.Println(s2_buf.Bytes())

    conn.Write(s2_buf.Bytes())

    return 0;
}

func handleConnect(stream *rtmpStream,trans float64,obj *amfObj) {
    fmt.Println("handleConnect")    

    stream.app = obj.objs["app"].str
    stream.state = RTMP_STREAM_CONNECTED
    //appMap[stream.app] = make(chan *rtmpPacket,16)
    var packet rtmpPacket
    packet.data = new(bytes.Buffer)

    amfObjs := []amfObj {
                amfObj {objType : AMF_STRING,str : "_result", },
                amfObj {objType : AMF_NUMBER, f64 : trans, },
                amfObj {objType : AMF_OBJECT,
                        objs : map[string] *amfObj {
                                "fmtVer" : &amfObj {objType : AMF_STRING, str : "FMS/3,0,1,123", },
                                "capabilities" : &amfObj {objType : AMF_NUMBER, f64 : 31, },
                        },
                },
                amfObj {objType : AMF_OBJECT,
                        objs : map[string] *amfObj {
                                "level" : &amfObj {objType : AMF_STRING, str : "status", },
                                "code" : &amfObj {objType : AMF_STRING, str : "NetConnection.Connect.Success", },
                                "description" : &amfObj {objType : AMF_STRING, str : "Connection Success.", },
                                "objectEncoding" : &amfObj {objType : AMF_NUMBER, f64 : 0, },
                        },
                },
        }

    packet.csid = 3
    packet.typeid = MSG_AMF_CMD

    for _,o := range(amfObjs) {
        binary.Write(packet.data,binary.BigEndian,amf_encode(&o))
    }

    fmt.Println(packet.data.Bytes())
    
    stream.writeChunkPacket(&packet)
}

func handleServerBW(stream *rtmpStream,packet *rtmpPacket) {
    fmt.Println("handleServerBW")    
    
    var response rtmpPacket
    response.data = new(bytes.Buffer)
    response.typeid = MSG_BANDWIDTH
    srvBW := &amfObj{objType : AMF_NUMBER,f64 : 5000000}
    binary.Write(response.data,binary.BigEndian,amf_encode(srvBW))
    fmt.Println(response.data.Bytes())
    stream.writeChunkPacket(&response)
}

func handleCreateStream(stream *rtmpStream,obj *amfObj) {
    fmt.Println("handleCreateStream")

    var packet rtmpPacket
    packet.data = new(bytes.Buffer)

    amfObjs := []amfObj {
                amfObj {objType : AMF_STRING,str : "_result", },
                amfObj {objType : AMF_NUMBER, f64 : obj.f64, },
                amfObj {objType : AMF_NULL,},
                amfObj {objType : AMF_NUMBER, f64 : 1, },
        }

    packet.csid = 3
    packet.typeid = MSG_AMF_CMD

    for _,o := range(amfObjs) {
        binary.Write(packet.data,binary.BigEndian,amf_encode(&o))
    }

    fmt.Println(packet.data.Bytes())
    
    stream.writeChunkPacket(&packet)
}

func handlePublish(stream *rtmpStream,obj *amfObj) {
    fmt.Println("handlePublish")

    if _,ok := appMap[stream.app]; ok {
        fmt.Printf("app %s already exists",stream.app)
        stream.Close()
        return
    } 

    stream.role = RTMP_STREAM_PUBLISHER
    stream.state = RTMP_STREAM_ACTIVE
    appMap[stream.app] = make(chan *rtmpPacket,16)

    var packet rtmpPacket
    packet.data = new(bytes.Buffer)

    amfObjs := []amfObj {
                amfObj {objType : AMF_STRING,str : "onStatus", },
                amfObj {objType : AMF_NUMBER, f64 : obj.f64, },
                amfObj {objType : AMF_NULL, },
                amfObj {objType : AMF_OBJECT,
                        objs : map[string] *amfObj {
                                "level" : &amfObj {objType : AMF_STRING, str : "status", },
                                "code" : &amfObj {objType : AMF_STRING, str : "NetStream.Publish.Start", },
                                "description" : &amfObj {objType : AMF_STRING, str : "Start publising.", },
                        },
                },
        }

    packet.csid = 3
    packet.typeid = MSG_AMF_CMD

    for _,o := range(amfObjs) {
        binary.Write(packet.data,binary.BigEndian,amf_encode(&o))
    }

    fmt.Println(packet.data.Bytes())
    
    stream.writeChunkPacket(&packet)
}

func handlePlay(stream *rtmpStream,data *rtmpPacket) {
    fmt.Println("handlePlay")

    stream.role = RTMP_STREAM_PLAYER
    stream.state = RTMP_STREAM_ACTIVE

    transid := amf_decode(data.data)
    _ = amf_decode(data.data)
    strname := amf_decode(data.data)
    fmt.Println("transid: ",transid.f64)
    fmt.Println("strname: ",strname.str)

    eventData := data.streamId
    var eventId int16 = 0

    buf := new(bytes.Buffer)
    binary.Write(buf,binary.BigEndian,eventId)
    binary.Write(buf,binary.LittleEndian,eventData)
    stream.streamBegin(data.csid,data.streamId,buf.Bytes())

    var packet rtmpPacket
    packet.data = new(bytes.Buffer)

    amfObjs := []amfObj {
                amfObj {objType : AMF_STRING,str : "onStatus", },
                amfObj {objType : AMF_NUMBER, f64 : 0, },
                amfObj {objType : AMF_NULL, },
                amfObj {objType : AMF_OBJECT,
                        objs : map[string] *amfObj {
                                "level" : &amfObj {objType : AMF_STRING, str : "status", },
                                "code" : &amfObj {objType : AMF_STRING, str : "NetStream.Play.Start", },
                                "description" : &amfObj {objType : AMF_STRING, str : "Start live.", },
                        },
                },
    }

    packet.csid = data.csid
    packet.typeid = MSG_AMF_CMD
    for _,o := range(amfObjs) {
        binary.Write(packet.data,binary.BigEndian,amf_encode(&o))
    }
    fmt.Println(packet.data.Bytes())
    stream.writeChunkPacket(&packet)


    amfObjs = []amfObj {
        amfObj {objType : AMF_STRING,str : "|RtmpSampleAccess", },
        amfObj {objType : AMF_BOOLEAN, i : 1, },
        amfObj {objType : AMF_BOOLEAN, i : 1, },
    }
    packet.data.Reset()
    packet.csid = data.csid
    packet.typeid = MSG_AMF_META
    for _,o := range(amfObjs) {
        binary.Write(packet.data,binary.BigEndian,amf_encode(&o))
    }
    fmt.Println(packet.data.Bytes())
    stream.writeChunkPacket(&packet)
    
    
    amfObjs = []amfObj {
        amfObj {objType : AMF_STRING,str : "onMetaData", },
        amfObj {objType : AMF_OBJECT,
            objs : map[string] *amfObj {
                                "server" : &amfObj {objType : AMF_STRING, str : "Golang Rtmp Server", },
                                "width" : &amfObj {objType : AMF_NUMBER, f64 : 300, },
                                "height" : &amfObj {objType : AMF_NUMBER, f64 : 240, },
                                "displayWidth" : &amfObj {objType : AMF_NUMBER, f64 : 300, },
                                "displayHeight" : &amfObj {objType : AMF_NUMBER, f64 : 240, },
                                "duration" : &amfObj {objType : AMF_NUMBER, f64 : 0, },
                                "videodatarate" : &amfObj {objType : AMF_NUMBER, f64 : 731, },
                                "videocodecid" : &amfObj {objType : AMF_NUMBER, f64 : 2, },
                        },
        },
    }

    packet.data.Reset()
    packet.csid = data.csid
    packet.typeid = MSG_AMF_META
    for _,o := range(amfObjs) {
        binary.Write(packet.data,binary.BigEndian,amf_encode(&o))
    }
    fmt.Println(packet.data.Bytes())
    stream.writeChunkPacket(&packet)
    

    if app,ok := appMap[stream.app]; ok {
        nr := 0
        for {
            vv := <- app
            if vv == nil {
                break
            }
            if nr == 0 {
                stream.writeVideoPacket(0,data.csid,data.streamId,vv)
                nr++
                continue
            }
            stream.writeVideoPacket(1,data.csid,data.streamId,vv)
        }
    }

    eventId = 1
    buf.Reset()
    binary.Write(buf,binary.BigEndian,eventId)
    binary.Write(buf,binary.LittleEndian,eventData)
    stream.streamEnd(data.csid,data.streamId,buf.Bytes())
}

func handleDeleteStream(stream *rtmpStream) {
    if stream.role == RTMP_STREAM_PUBLISHER {
        if app,ok := appMap[stream.app]; ok {
            app <- nil
            delete(appMap,stream.app)
        }
    }
}

func handleInvode(stream *rtmpStream,packet *rtmpPacket) {
    fmt.Println("handleInvode")
    obj := amf_decode(packet.data)
    switch obj.str {
    case "connect":
        obj2 := amf_decode(packet.data)
        obj3 := amf_decode(packet.data)
        handleConnect(stream,obj2.f64,obj3)
    case "createStream":
        obj2 := amf_decode(packet.data)
        handleCreateStream(stream,obj2)
    case "publish":
        handlePublish(stream,obj)
    case "play":
        handlePlay(stream,packet)
    case "deleteStream":
        handleDeleteStream(stream)
    default:
        fmt.Println("error command from client")
    }
}

func handleAudio(stream *rtmpStream,packet *rtmpPacket) {
    fmt.Println("handleAudio")

    if app,ok := appMap[stream.app]; ok {
        app <- packet
    }
}

func handleVideo(stream *rtmpStream,packet *rtmpPacket) {
    fmt.Println("handleVideo")

    if app,ok := appMap[stream.app]; ok {
        app <- packet
    }
}

func handle(conn net.Conn) {
    fmt.Println("new connection")

    stream := &rtmpStream{conn:conn,state:RTMP_STREAM_INIT,msgs:map[int32]*rtmpPacket{}}
    defer stream.Close()
    defer func() {
            if x := recover(); x != nil {
                if stream.state == RTMP_STREAM_ACTIVE {
                    if stream.role == RTMP_STREAM_PUBLISHER {
                        if app,ok := appMap[stream.app]; ok {
                            app <- nil
                            delete(appMap,stream.app)
                        }
                    }
                }
            }
    }()

    if handShanke(conn) < 0 {
        fmt.Println("rtmp handshake failed")
        return
    }

    for {
        packet := stream.readChunkPacket()
        if packet == nil {
            continue
        }
        switch packet.typeid {
        case MSG_AMF_CMD:
            handleInvode(stream,packet)
        case MSG_BANDWIDTH:
            handleServerBW(stream,packet)
        case MSG_VIDEO:
            handleVideo(stream,packet)
        case MSG_AUDIO:
            handleAudio(stream,packet)
        }
    }
}

//StreamID=(ChannelID-4)/5+1
func Serve() {
    runtime.GOMAXPROCS(runtime.NumCPU())

    ln,err := net.Listen("tcp",":1935")
    if err != nil {
        fmt.Println(err)
        return
    }
    
    for {
        conn, err := ln.Accept()
        if err != nil {
            continue
        }
        go handle(conn)
    }
}