#/bin/sh

ethermint --datadir ~/.ethermint  --networkid 30308 --port 30308 --rpc   --rpcport 6969   --rpcaddr=0.0.0.0   --rpccorsdomain=*   --rpcapi "eth,web3,personal,net,admin"  --verbosity=6 --log_level=dbug> ethereum.log 2>&1 &
