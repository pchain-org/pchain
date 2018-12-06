#/bin/sh

./bin/pchain --datadir ./.pchain  --networkid 30308 --port 30308 --mine --syncmode "full"  --gcmode=archive --rpc   --rpcport 6969   --rpcaddr=0.0.0.0   --rpccorsdomain=*   --rpcapi "eth,web3,personal,net,admin,miner,tdm"  --verbosity=6 > pchain.log 2>&1