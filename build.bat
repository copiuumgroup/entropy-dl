@echo off
setlocal enabledelayedexpansion

:: ═══════════════════════════════════════════════════════════════════════
::  Entropy DL — Build Script (Windows)
::  Usage: build.bat [release|clean]
:: ═══════════════════════════════════════════════════════════════════════

set "APP_DIR=%~dp0"
set "FRONTEND_DIR=%APP_DIR%frontend"
set "BACKEND_DIR=%APP_DIR%backend"
set "RELEASES_DIR=%APP_DIR%releases"
set "BINARY_NAME=entropy.exe"

:: ─── Version ───
set "VERSION=dev"
if exist "%APP_DIR%VERSION" (
  set /p VERSION=<"%APP_DIR%VERSION"
  set "VERSION=!VERSION: =!"
)

set "COMMAND=%~1"
if "%COMMAND%"=="" set "COMMAND=release"

if /i "%COMMAND%"=="clean" goto :clean
if /i "%COMMAND%"=="release" goto :release

echo Usage: %~nx0 [release^|clean]
goto :eof

:: ───────────────────────────────────────────────────────────────────
:release
echo === Entropy DL v%VERSION% - Release Build ===
echo.

:: 1. Install frontend deps
echo   [1/5] Installing frontend dependencies...
cd /d "%FRONTEND_DIR%"
call npm.cmd install --legacy-peer-deps --silent
if errorlevel 1 (
  echo   ERROR: npm install failed
  exit /b 1
)

:: 2. Build frontend
echo   [2/5] Building frontend (tsc + vite)...
call npm.cmd run build --silent
if errorlevel 1 (
  echo   ERROR: Frontend build failed
  exit /b 1
)
echo     Frontend built -^> frontend\build\

:: 3. Copy to webdist
echo   [3/5] Copying frontend -^> backend\webdist...
cd /d "%APP_DIR%"
if exist "%BACKEND_DIR%\webdist" rmdir /s /q "%BACKEND_DIR%\webdist"
xcopy /e /i /q "%FRONTEND_DIR%\build" "%BACKEND_DIR%\webdist\" >nul

:: 4. Build Go binary
echo   [4/5] Building Go binary...
cd /d "%BACKEND_DIR%"
go build -ldflags "-s -w -X main.version=%VERSION%" -o "%BINARY_NAME%" .
if errorlevel 1 (
  echo   ERROR: Go build failed
  exit /b 1
)
echo     Binary built -^> backend\%BINARY_NAME%

:: 5. Package into releases/
echo   [5/5] Packaging release...
if not exist "%RELEASES_DIR%" mkdir "%RELEASES_DIR%"
set "ARCHIVE_NAME=entropy-v%VERSION%-windows-amd64"
set "ARCHIVE_DIR=%RELEASES_DIR%\entropy-v%VERSION%"
if exist "%ARCHIVE_DIR%" rmdir /s /q "%ARCHIVE_DIR%"
mkdir "%ARCHIVE_DIR%"
copy "%BACKEND_DIR%\%BINARY_NAME%" "%ARCHIVE_DIR%\" >nul

:: Also copy raw binary directly into releases/
copy "%BACKEND_DIR%\%BINARY_NAME%" "%RELEASES_DIR%\%BINARY_NAME%" >nul

echo.
echo   ========================================
echo   Build complete
echo     Binary:  %BINARY_NAME%
echo     Release: releases\%ARCHIVE_NAME%\
echo   ========================================
echo.
echo   Run:  .\releases\%BINARY_NAME%
echo.
goto :eof

:: ───────────────────────────────────────────────────────────────────
:clean
echo === Entropy // Clean ===
if exist "%FRONTEND_DIR%\build" rmdir /s /q "%FRONTEND_DIR%\build"
if exist "%BACKEND_DIR%\webdist" rmdir /s /q "%BACKEND_DIR%\webdist"
if exist "%RELEASES_DIR%" rmdir /s /q "%RELEASES_DIR%"
if exist "%BACKEND_DIR%\entropy.exe" del /q "%BACKEND_DIR%\entropy.exe"
echo   Done.
goto :eof