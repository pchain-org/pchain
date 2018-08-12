#/bin/sh

pchain --datadir /home/ubuntu/.pchain  --networkid 30308 --port 30308 --rpc   --rpcport 6969   --rpcaddr=0.0.0.0   --rpccorsdomain=*   --rpcapi "eth,web3,personal,net,admin" &
