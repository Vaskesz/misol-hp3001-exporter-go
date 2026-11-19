@echo off
setlocal
set NAME=misol-hp3001-exporter-go
set OUTDIR=build
set VERSION=1.0.0

rem --- Determine git commit hash (optional) ---
set GIT_COMMIT=unknown
for /f "delims=" %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i

rem --- Build date in ISO 8601 without spaces (safe for cmd quoting) ---
for /f "tokens=1-3 delims=/-. " %%a in ("%DATE%") do set TODAY=%%c-%%b-%%a
set BUILD_DATE=%TODAY%T%TIME:~0,2%:%TIME:~3,2%:%TIME:~6,2%Z
set BUILD_DATE=%BUILD_DATE: =0%

rem --- Common linker flags ---
set LDFLAGS=-s -w -X main.Version=%VERSION% -X main.Commit=%GIT_COMMIT% -X main.BuildDate=%BUILD_DATE%

if not exist "%OUTDIR%" mkdir "%OUTDIR%"
if not exist "%OUTDIR%" (
    echo Failed to create output directory "%OUTDIR%".
    goto :error
)

set CGO_ENABLED=0

rem ============================================================
rem Windows (native)
rem ============================================================
set GOOS=windows
set GOARCH=amd64
set GOARM=
go build -ldflags "%LDFLAGS%" -o "%OUTDIR%\%NAME%.exe" || goto :error

rem ============================================================
rem Linux on Raspberry Pi
rem ============================================================
set GOOS=linux

set GOARCH=arm64
set GOARM=
go build -ldflags "%LDFLAGS%" -o "%OUTDIR%\%NAME%-linux-arm64" || goto :error

set GOARCH=arm
set GOARM=7
go build -ldflags "%LDFLAGS%" -o "%OUTDIR%\%NAME%-linux-armv7" || goto :error

rem ============================================================
rem Linux on regular PC (Intel / AMD)
rem ============================================================
set GOARM=

set GOARCH=amd64
go build -ldflags "%LDFLAGS%" -o "%OUTDIR%\%NAME%-linux-amd64" || goto :error

set GOARCH=386
go build -ldflags "%LDFLAGS%" -o "%OUTDIR%\%NAME%-linux-386" || goto :error

echo All builds succeeded. Version: %VERSION%, commit: %GIT_COMMIT%
endlocal
exit /b 0

:error
echo Build failed.
endlocal
exit /b 1