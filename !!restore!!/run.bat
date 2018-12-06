
@echo off

set PWD=%~dp0
%PWD%\bin\pchain.exe --datadir ./.pchain  --networkid 30308 --port 30308 --mine --syncmode "full"  --rpc   --rpcport 6969   --rpcaddr=0.0.0.0   --rpccorsdomain=*   --rpcapi "eth,web3,personal,net,admin,miner,tdm"  --verbosity=2 > pchain.log 2>&1
