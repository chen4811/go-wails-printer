@echo off
chcp 65001 >nul
echo ========================================
echo   打印服务器 - 开发模式
echo ========================================
echo.

REM 检查 PDFtoPrinter.exe
if exist PDFtoPrinter.exe (
    echo [OK] PDFtoPrinter.exe 存在
) else (
    echo [警告] 未找到 PDFtoPrinter.exe
    echo        PDF 打印功能将不可用
    echo        请从以下地址下载:
    echo        https://github.com/emendelson/pdftoprinter
    echo.
)

echo 启动开发服务器...
wails dev
