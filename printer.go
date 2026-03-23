package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// PrinterService 打印服务
type PrinterService struct {
	printers []PrinterInfo
	mu       sync.RWMutex
}

// NewPrinterService 创建打印服务
func NewPrinterService() *PrinterService {
	p := &PrinterService{}
	p.loadPrinters()
	return p
}

// loadPrinters 加载打印机列表
func (p *PrinterService) loadPrinters() {
	printers, err := p.getSystemPrinters()
	if err != nil {
		fmt.Printf("获取打印机列表失败: %v\n", err)
		printers = []PrinterInfo{}
	}
	p.printers = printers
	fmt.Printf("已加载 %d 台打印机\n", len(printers))
}

// getSystemPrinters 获取系统打印机列表
func (p *PrinterService) getSystemPrinters() ([]PrinterInfo, error) {
	switch runtime.GOOS {
	case "windows":
		return p.getWindowsPrinters()
	case "darwin":
		return p.getMacPrinters()
	case "linux":
		return p.getLinuxPrinters()
	default:
		return nil, errors.New("不支持的操作系统")
	}
}

// getWindowsPrinters 获取 Windows 打印机列表
func (p *PrinterService) getWindowsPrinters() ([]PrinterInfo, error) {
	cmd := exec.Command("powershell", "-Command", "Get-Printer | Select-Object Name | ForEach-Object { $_.Name }")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var printers []PrinterInfo
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			printers = append(printers, PrinterInfo{
				Name:      name,
				IsDefault: false,
				Status:    "ready",
			})
		}
	}

	return printers, nil
}

// getMacPrinters 获取 Mac 打印机列表
func (p *PrinterService) getMacPrinters() ([]PrinterInfo, error) {
	// 获取打印机列表
	cmd := exec.Command("lpstat", "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var printers []PrinterInfo
	lines := strings.Split(string(output), "\n")

	// 获取默认打印机
	defaultPrinter := ""
	cmdDefault := exec.Command("lpstat", "-d")
	outputDefault, err := cmdDefault.Output()
	if err == nil {
		// 格式: "系统默认目的位置：HP_Laser_MFP_1188nw__41_73_5C_" 或 "system default destination: HP_xxx"
		outputStr := strings.TrimSpace(string(outputDefault))
		// 查找冒号后的内容
		if idx := strings.LastIndex(outputStr, "："); idx != -1 {
			defaultPrinter = strings.TrimSpace(outputStr[idx+3:]) // 中文冒号占3字节
		} else if idx := strings.LastIndex(outputStr, ":"); idx != -1 {
			defaultPrinter = strings.TrimSpace(outputStr[idx+1:])
		}
		fmt.Printf("默认打印机: '%s'\n", defaultPrinter)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 格式: "打印机HP_Laser_MFP_1188nw__41_73_5C_闲置，启用时间始于..." 
		// 或 "printer HP_xxx is idle..."
		var name string
		
		if strings.HasPrefix(line, "打印机") {
			// 中文格式: "打印机<name><status>"
			rest := strings.TrimPrefix(line, "打印机")
			// 打印机名称是连续的非中文字符，遇到中文字符（状态）就停止
			name = extractPrinterName(rest)
		} else if strings.HasPrefix(line, "printer") {
			// 英文格式: "printer <name> <status>"
			rest := strings.TrimPrefix(line, "printer")
			rest = strings.TrimSpace(rest)
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				name = fields[0]
			}
		}

		if name != "" {
			printers = append(printers, PrinterInfo{
				Name:      name,
				IsDefault: name == defaultPrinter,
				Status:    "ready",
			})
		}
	}

	return printers, nil
}

// extractPrinterName 从字符串中提取打印机名称（遇到中文字符停止）
func extractPrinterName(s string) string {
	result := ""
	for _, r := range s {
		// 如果是中文字符，停止
		if r >= 0x4e00 && r <= 0x9fff {
			break
		}
		// 如果是空格或制表符，停止（英文格式）
		if r == ' ' || r == '\t' {
			break
		}
		result += string(r)
	}
	return result
}

// getLinuxPrinters 获取 Linux 打印机列表
func (p *PrinterService) getLinuxPrinters() ([]PrinterInfo, error) {
	cmd := exec.Command("lpstat", "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var printers []PrinterInfo
	lines := strings.Split(string(output), "\n")

	// 获取默认打印机
	defaultPrinter := ""
	cmdDefault := exec.Command("lpstat", "-d")
	outputDefault, err := cmdDefault.Output()
	if err == nil {
		parts := strings.Split(string(outputDefault), ":")
		if len(parts) > 1 {
			defaultPrinter = strings.TrimSpace(parts[1])
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "printer") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				name := fields[1]
				printers = append(printers, PrinterInfo{
					Name:      name,
					IsDefault: name == defaultPrinter,
					Status:    "ready",
				})
			}
		}
	}

	return printers, nil
}

// GetPrinters 获取打印机列表
func (p *PrinterService) GetPrinters() ([]PrinterInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.printers, nil
}

// Print 执行打印
func (p *PrinterService) Print(data []byte, fileType string, printerName string) error {
	// 创建临时文件
	tempDir := os.TempDir()
	ext := p.getFileExtension(fileType)
	tempFile := filepath.Join(tempDir, fmt.Sprintf("print_%d_%d%s", os.Getpid(), time.Now().UnixNano(), ext))

	// 写入临时文件
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %v", err)
	}
	defer os.Remove(tempFile)

	// 根据文件类型处理
	switch strings.ToLower(fileType) {
	case "pdf":
		return p.printPDF(tempFile, printerName)
	case "image", "jpg", "jpeg", "png", "gif", "bmp", "webp":
		return p.printImage(tempFile, printerName)
	case "word", "doc", "docx", "excel", "xls", "xlsx":
		return errors.New("不支持打印 Word/Excel 文件")
	default:
		return p.printFile(tempFile, printerName)
	}
}

// printPDF 打印 PDF
func (p *PrinterService) printPDF(filePath string, printerName string) error {
	switch runtime.GOOS {
	case "windows":
		return p.printPDFWindows(filePath, printerName)
	case "darwin":
		cmd := exec.Command("lpr")
		if printerName != "" {
			cmd.Args = append(cmd.Args, "-P", printerName)
		}
		cmd.Args = append(cmd.Args, filePath)
		return cmd.Run()
	case "linux":
		cmd := exec.Command("lpr")
		if printerName != "" {
			cmd.Args = append(cmd.Args, "-P", printerName)
		}
		cmd.Args = append(cmd.Args, filePath)
		return cmd.Run()
	default:
		return errors.New("不支持的操作系统")
	}
}

// printPDFWindows Windows 平台打印 PDF
func (p *PrinterService) printPDFWindows(filePath string, printerName string) error {
	// 方法1: 使用 Shell.Application COM 对象打印 (最可靠)
	psScript := fmt.Sprintf(`
$sh = New-Object -ComObject Shell.Application
$folder = $sh.Namespace((Split-Path -Path '%s' -Parent))
$file = $folder.ParseName((Split-Path -Path '%s' -Leaf))
%s
$file.InvokeVerb('Print')
Start-Sleep -Seconds 3
`, filePath, filePath, p.getPrinterSetupScript(printerName))

	cmd := exec.Command("powershell", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	fmt.Printf("Shell.Application 打印失败: %v, 输出: %s\n", err, string(output))

	// 方法2: 使用 Adobe Reader 或 Edge 打印
	if err := p.printWithAdobeReader(filePath, printerName); err == nil {
		return nil
	}
	fmt.Printf("Adobe Reader 打印失败: %v\n", err)

	// 方法3: 使用 Ghostscript (如果安装)
	if err := p.printWithGhostscript(filePath, printerName); err == nil {
		return nil
	}
	fmt.Printf("Ghostscript 打印失败: %v\n", err)

	// 方法4: 最后尝试用默认程序打开，用户手动打印
	fmt.Println("所有自动打印方法失败，尝试打开文件...")
	cmd = exec.Command("cmd", "/c", "start", "", filePath)
	return cmd.Run()
}

// getPrinterSetupScript 获取打印机设置脚本
func (p *PrinterService) getPrinterSetupScript(printerName string) string {
	if printerName == "" {
		return ""
	}
	// 获取当前默认打印机并临时更改
	return fmt.Sprintf(`
$oldDefault = (Get-WmiObject -Query "SELECT * FROM Win32_Printer WHERE Default=$true").Name
$targetPrinter = Get-WmiObject -Query "SELECT * FROM Win32_Printer WHERE Name='%s'"
if ($targetPrinter) { $targetPrinter.SetDefaultPrinter() }
`, printerName)
}

// printWithAdobeReader 使用 Adobe Reader 打印
func (p *PrinterService) printWithAdobeReader(filePath string, printerName string) error {
	// 查找 Adobe Reader 路径
	adobePaths := []string{
		`C:\Program Files\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
		`C:\Program Files (x86)\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
		`C:\Program Files\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
		`C:\Program Files (x86)\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
		`C:\Program Files\Adobe\Reader 11.0\Reader\AcroRd32.exe`,
		`C:\Program Files (x86)\Adobe\Reader 11.0\Reader\AcroRd32.exe`,
	}

	for _, adobePath := range adobePaths {
		if _, err := os.Stat(adobePath); err == nil {
			var cmd *exec.Cmd
			if printerName != "" {
				cmd = exec.Command(adobePath, "/t", filePath, printerName)
			} else {
				cmd = exec.Command(adobePath, "/t", filePath)
			}
			return cmd.Run()
		}
	}
	return errors.New("未找到 Adobe Reader")
}

// printWithGhostscript 使用 Ghostscript 打印
func (p *PrinterService) printWithGhostscript(filePath string, printerName string) error {
	// 查找 Ghostscript 路径
	gsPaths := []string{
		`C:\Program Files\gs\gs10.02.1\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.01.1\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.56.1\bin\gswin64c.exe`,
		`C:\Program Files (x86)\gs\gs9.56.1\bin\gswin32c.exe`,
	}

	// 也可以尝试 PATH 中的 gswin64c
	gsInPath, _ := exec.LookPath("gswin64c.exe")
	if gsInPath != "" {
		gsPaths = append([]string{gsInPath}, gsPaths...)
	}

	for _, gsPath := range gsPaths {
		if _, err := os.Stat(gsPath); err == nil {
			printer := printerName
			if printer == "" {
				// 获取默认打印机
				cmd := exec.Command("powershell", "-Command", "(Get-WmiObject -Query \"SELECT * FROM Win32_Printer WHERE Default=$true\").Name")
				output, _ := cmd.Output()
				printer = strings.TrimSpace(string(output))
			}
			if printer == "" {
				continue
			}
			cmd := exec.Command(gsPath,
				"-dPrinted", "-dBATCH", "-dNOPAUSE",
				"-dNoCancel", "-q",
				"-sDEVICE=mswinpr2",
				fmt.Sprintf(`-sOutputFile="\\spool\%s"`, printer),
				filePath)
			return cmd.Run()
		}
	}
	return errors.New("未找到 Ghostscript")
}

// printImage 打印图片
func (p *PrinterService) printImage(filePath string, printerName string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("mspaint", "/pt", filePath)
		return cmd.Run()
	case "darwin":
		cmd := exec.Command("lpr")
		if printerName != "" {
			cmd.Args = append(cmd.Args, "-P", printerName)
		}
		cmd.Args = append(cmd.Args, filePath)
		return cmd.Run()
	case "linux":
		cmd := exec.Command("lpr")
		if printerName != "" {
			cmd.Args = append(cmd.Args, "-P", printerName)
		}
		cmd.Args = append(cmd.Args, filePath)
		return cmd.Run()
	default:
		return errors.New("不支持的操作系统")
	}
}

// printFile 打印普通文件
func (p *PrinterService) printFile(filePath string, printerName string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("print")
		if printerName != "" {
			cmd.Args = append(cmd.Args, "/D:"+printerName)
		}
		cmd.Args = append(cmd.Args, filePath)
		return cmd.Run()
	case "darwin", "linux":
		cmd := exec.Command("lpr")
		if printerName != "" {
			cmd.Args = append(cmd.Args, "-P", printerName)
		}
		cmd.Args = append(cmd.Args, filePath)
		return cmd.Run()
	default:
		return errors.New("不支持的操作系统")
	}
}

// getFileExtension 获取文件扩展名
func (p *PrinterService) getFileExtension(fileType string) string {
	switch strings.ToLower(fileType) {
	case "pdf":
		return ".pdf"
	case "image", "jpg", "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	case "bmp":
		return ".bmp"
	case "webp":
		return ".webp"
	default:
		return ".bin"
	}
}

// decodeBase64 解码 Base64 数据
func decodeBase64(data string) ([]byte, error) {
	// 移除 data URL 前缀
	if strings.Contains(data, ",") {
		parts := strings.Split(data, ",")
		data = parts[1]
	}

	// 解码
	return base64.StdEncoding.DecodeString(data)
}

// DownloadFile 下载远程文件并返回文件数据和类型
func (p *PrinterService) DownloadFile(url string) ([]byte, string, error) {
	// 创建 HTTP 客户端，设置超时
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	
	// 发送 GET 请求
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP 错误: %d", resp.StatusCode)
	}
	
	// 读取文件内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %v", err)
	}
	
	// 检测文件类型
	fileType := detectFileType(url, resp.Header.Get("Content-Type"), data)
	
	return data, fileType, nil
}

// detectFileType 检测文件类型
func detectFileType(url string, contentType string, data []byte) string {
	// 先从 URL 扩展名判断
	urlLower := strings.ToLower(url)
	if strings.Contains(urlLower, ".pdf") {
		return "pdf"
	}
	if strings.Contains(urlLower, ".jpg") || strings.Contains(urlLower, ".jpeg") {
		return "jpg"
	}
	if strings.Contains(urlLower, ".png") {
		return "png"
	}
	if strings.Contains(urlLower, ".gif") {
		return "gif"
	}
	if strings.Contains(urlLower, ".bmp") {
		return "bmp"
	}
	if strings.Contains(urlLower, ".webp") {
		return "webp"
	}
	
	// 从 Content-Type 判断
	ctLower := strings.ToLower(contentType)
	if strings.Contains(ctLower, "pdf") {
		return "pdf"
	}
	if strings.Contains(ctLower, "jpeg") || strings.Contains(ctLower, "jpg") {
		return "jpg"
	}
	if strings.Contains(ctLower, "png") {
		return "png"
	}
	if strings.Contains(ctLower, "gif") {
		return "gif"
	}
	if strings.Contains(ctLower, "bmp") {
		return "bmp"
	}
	if strings.Contains(ctLower, "webp") {
		return "webp"
	}
	if strings.HasPrefix(ctLower, "image/") {
		return "image"
	}
	
	// 从文件头判断
	if len(data) > 4 {
		// PDF: %PDF
		if data[0] == 0x25 && data[1] == 0x50 && data[2] == 0x44 && data[3] == 0x46 {
			return "pdf"
		}
		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "png"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "jpg"
		}
		// GIF: GIF8
		if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
			return "gif"
		}
		// BMP: BM
		if data[0] == 0x42 && data[1] == 0x4D {
			return "bmp"
		}
		// WebP: RIFF....WEBP
		if len(data) > 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
			data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "webp"
		}
	}
	
	// 默认返回 PDF
	return "pdf"
}
