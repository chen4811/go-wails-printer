@echo off
chcp 65001 >nul
echo ========================================
echo   打印服务器 - 构建应用
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

echo 开始构建...
wails build

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [成功] 构建完成!
    
    REM 复制 PDFtoPrinter.exe 到输出目录
    if exist PDFtoPrinter.exe (
        if not exist build\bin mkdir build\bin
        copy /Y PDFtoPrinter.exe build\bin\ >nul
        echo [OK] PDFtoPrinter.exe 已复制到 build\bin\
    )
    
    REM 复制图标文件到输出目录
    if exist build\windows\icon.ico (
        copy /Y build\windows\icon.ico build\bin\icon.ico >nul
        echo [OK] icon.ico 已复制到 build\bin\
    )
    
    echo.
    echo 输出目录: build\bin\
    echo 可执行文件: 打印服务器.exe
) else (
    echo.
    echo [错误] 构建失败!
)

pause
