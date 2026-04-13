package systool

import (
    "bytes"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"
    "syscall"
    "time"
    "unsafe"

    "golang.org/x/sys/windows"
)

var chcp65001done bool

func CmdOut(bin string, args ...interface{}) []byte {

    // todo 别的系统不需要
    if !chcp65001done {
        // utf8 中文支持
        // Change the code page to Unicode/65001
        exec.Command("chcp", "65001").Run()
        chcp65001done = true
    }

    var strs []string
    for _, arg := range args {
        switch value := arg.(type) {
        case string:
            strs = append(strs, value)
        case []string:
            strs = append(strs, value...)
        default:
            log.Fatalln("CmdOut", "not string argument", arg)
        }
    }
    cmd := exec.Command(bin, strs...)
    // todo 其他系统没有 HideWindow 属性
    cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
    out, err := cmd.Output()
    if nil != err {
        panic(err.Error() + string(out))
    }

    return bytes.TrimSpace(out)
}

func CmdBat(runAsAdmin bool, lines ...string) []byte {
    file, err := os.Create(os.TempDir() + time.Now().Format("/cnfix-2006.01.02.15.04.05.000.bat"))
    if nil != err {
        log.Fatalln(err)
    }
    defer os.Remove(file.Name())

    // 换行符必须是 \r\n 否则 bat 执行出错
    _, err = file.WriteString(strings.Join(lines, "\r\n"))
    if nil != err {
        log.Fatalln(err)
    }
    file.Close()

    if runAsAdmin {
        return CmdOut(
            "powershell",
            fmt.Sprintf("Start-Process -Verb RunAs -FilePath '%s' -WindowStyle Hidden -Wait", file.Name()),
        )
    } else {
        return CmdOut(file.Name())
    }
}

// show_window:
//   windows.SW_SHOWNORMAL
//   windows.SW_HIDE
func RunAsAdmin(exe string, args string, show_window uintptr) error {
    shell32 := windows.NewLazySystemDLL("shell32.dll")
    shellExecute := shell32.NewProc("ShellExecuteW")

    verb, _ := syscall.UTF16PtrFromString("runas")
    file, _ := syscall.UTF16PtrFromString(exe)
    argPtr, _ := syscall.UTF16PtrFromString(args)

    ret, _, _ := shellExecute.Call(
        0,
        uintptr(unsafe.Pointer(verb)),
        uintptr(unsafe.Pointer(file)),
        uintptr(unsafe.Pointer(argPtr)),
        0,
        show_window,
    )

    if ret <= 32 {
        return fmt.Errorf("ShellExecute failed with code %d", ret)
    }
    return nil
}
