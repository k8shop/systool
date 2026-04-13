//go:build windows

package systool

import (
    "encoding/binary"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "time"

    "github.com/Microsoft/go-winio"
    "golang.org/x/sys/windows"
)

var AdminPipe = adminPipe{FuncMap: make(map[string]func(...any) (any, error))}

type adminPipe struct {
    listener net.Listener
    FuncMap  map[string]func(...any) (any, error)
}

var adminPipePath = `\\.\pipe\cnfix_runas_admin`

func AdminPipeListen() {
    listener, err := winio.ListenPipe(adminPipePath, &winio.PipeConfig{
        SecurityDescriptor: "D:P(A;;GA;;;WD)",
    })
    if err != nil {
        log.Println(err)
        return
    }
    defer listener.Close()
    AdminPipe.listener = listener

    log.Println("AdminPipe listening...")

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("Accept error:", err)
            break
        }
        go func() {
            defer conn.Close()

            var length [2]byte
            var res []byte
            var call []any
            var data any

            conn.SetDeadline(time.Now().Add(time.Minute))

            _, err = io.ReadFull(conn, length[:])
            if err != nil {
                goto end
            }

            res = make([]byte, binary.LittleEndian.Uint16(length[:]))
            _, err = io.ReadFull(conn, res)
            if err != nil {
                goto end
            }

            err = json.Unmarshal(res, &call)
            if err != nil {
                goto end
            }

            if fn, ok := AdminPipe.FuncMap[call[0].(string)]; ok {
                data, err = fn(call[1].([]any)...)
            } else {
                err = fmt.Errorf("%s: not found", call[0].(string))
            }

        end:
            var bs []byte
            if err == nil {
                bs, err = json.Marshal([]any{data, nil})
                if err != nil {
                    bs, _ = json.Marshal([]any{nil, err.Error()})
                }
            } else {
                bs, err = json.Marshal([]any{nil, err.Error()})
            }

            binary.LittleEndian.PutUint16(length[:], uint16(len(bs)))

            _, err = conn.Write(length[:])
            if err != nil {
                log.Println(err)
            }

            _, err = conn.Write(bs)
            if err != nil {
                log.Println(err)
            }
        }()
    }
}

func adminPipeRequest(input []any) (any, error) {
    conn, err := winio.DialPipe(adminPipePath, nil)
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    bs, err := json.Marshal(input)
    if err != nil {
        return nil, err
    }

    var length [2]byte
    binary.LittleEndian.PutUint16(length[:], uint16(len(bs)))

    conn.SetDeadline(time.Now().Add(time.Minute))

    _, err = conn.Write(length[:])
    if err != nil {
        return nil, err
    }

    _, err = conn.Write(bs)
    if err != nil {
        return nil, err
    }

    _, err = io.ReadFull(conn, length[:])
    if err != nil {
        return nil, err
    }

    res := make([]byte, binary.LittleEndian.Uint16(length[:]))
    _, err = io.ReadFull(conn, res)
    if err != nil {
        return nil, err
    }

    var arr []any
    err = json.Unmarshal(res, &arr)
    if err != nil {
        return nil, err
    }

    if arr[1] == nil {
        return arr[0], nil
    } else {
        return nil, errors.New(arr[1].(string))
    }
}

var adminPipeListening bool

func CallAsAdmin(name string, args ...any) (any, error) {
    if !adminPipeListening {
        if err := RunAsAdmin(os.Args[0], "InitAdminPipe", windows.SW_HIDE); err != nil {
            return nil, err
        }

        timeout := time.Millisecond * 10
        for i := 0; i < 150; i++ {
            conn, err := winio.DialPipe(adminPipePath, &timeout)
            if err != nil {
                time.Sleep(timeout)
                continue
            }
            conn.Close()
            adminPipeListening = true
            break
        }

        if !adminPipeListening {
            return nil, errors.New("InitAdminPipe: failed")
        }
    }

    if name == "adminPipe.close" {
        adminPipeListening = false
    }

    if args == nil {
        args = make([]any, 0)
    }

    return adminPipeRequest([]any{name, args})
}

func IsInitAdminPipe() bool {
    return len(os.Args) > 1 && os.Args[1] == "InitAdminPipe"
}

func init() {
    if IsInitAdminPipe() {
        AdminPipe.FuncMap["adminPipe.close"] = func(args ...any) (any, error) {
            return nil, AdminPipe.listener.Close()
        }
    }
}

/*
func main() {
    if systool.IsInitAdminPipe() {
        systool.AdminPipeListen()
        return
    }
}
*/
