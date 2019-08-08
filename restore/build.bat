@echo off

set PWD=%~dp0
::echo %PWD%
set workspace=%PWD%
::echo %workspace%
set pchaindir=%workspace%\src\github.com\pchain
::echo %pchaindir%

set GOPATH=%workspace%
::echo %GOPATH%
cd %pchaindir%
::echo %~dp0
go build -i -v -o %workspace%\bin\pchain.exe github.com/pchain/cmd
cd %workspace%
