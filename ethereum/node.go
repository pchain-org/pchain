package ethereum

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/core"
	"gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/consensus/tendermint"
)

var clientIdentifier = "geth" // Client identifier to advertise over the network

// MakeSystemNode sets up a local node and configures the services to launch
func MakeSystemNode(chainId, version string, ctx *cli.Context,
                    pNode tendermint.PChainP2P, cch core.CrossChainHelper) *node.Node {

	/*

	params.TargetGasLimit = common.String2Big(ctx.GlobalString(utils.TargetGasLimitFlag.Name))

	// Configure the node's service container
	stackConf := &node.Config{
		DataDir:     filepath.Join(utils.MakeDataDir(ctx), chainId),
		PrivateKey:  utils.MakeNodeKey(ctx),
		Name:        clientIdentifier,
		IPCPath:     utils.MakeIPCPath(ctx),
		HTTPHost:    utils.MakeHTTPRpcHost(ctx),
		HTTPPort:    ctx.GlobalInt(utils.RPCPortFlag.Name),
		HTTPCors:    ctx.GlobalString(utils.RPCCORSDomainFlag.Name),
		HTTPModules: utils.MakeRPCModules(ctx.GlobalString(utils.RPCApiFlag.Name)),
		WSHost:      utils.MakeWSRpcHost(ctx),
		WSPort:      ctx.GlobalInt(utils.WSPortFlag.Name),
		WSOrigins:   ctx.GlobalString(utils.WSAllowedOriginsFlag.Name),
		WSModules:   utils.MakeRPCModules(ctx.GlobalString(utils.WSApiFlag.Name)),
		NoDiscovery: true,
		MaxPeers: 0,
	}
	// Assemble and return the protocol stack
	stack, err := node.New(stackConf)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	// Configure the Ethereum service
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	// jitEnabled := ctx.GlobalBool(utils.VMEnableJitFlag.Name)
	ethConf := &eth.Config{
		ChainConfig: utils.MakeChainConfigWithPChainId(ctx, stack, chainId),
		// BlockChainVersion:       ctx.GlobalInt(utils.BlockchainVersionFlag.Name), TODO
		DatabaseCache:   ctx.GlobalInt(utils.CacheFlag.Name),
		DatabaseHandles: utils.MakeDatabaseHandles(),
		NetworkId:       ctx.GlobalInt(utils.NetworkIdFlag.Name),
		Etherbase:       utils.MakeEtherbase(ks, ctx),
		//EnableJit:               jitEnabled, // TODO
		//ForceJit:                ctx.GlobalBool(utils.VMForceJitFlag.Name),
		GasPrice:                common.String2Big(ctx.GlobalString(utils.GasPriceFlag.Name)),
		GpoMinGasPrice:          common.String2Big(ctx.GlobalString(utils.GpoMinGasPriceFlag.Name)),
		GpoMaxGasPrice:          common.String2Big(ctx.GlobalString(utils.GpoMaxGasPriceFlag.Name)),
		GpoFullBlockRatio:       ctx.GlobalInt(utils.GpoFullBlockRatioFlag.Name),
		GpobaseStepDown:         ctx.GlobalInt(utils.GpobaseStepDownFlag.Name),
		GpobaseStepUp:           ctx.GlobalInt(utils.GpobaseStepUpFlag.Name),
		GpobaseCorrectionFactor: ctx.GlobalInt(utils.GpobaseCorrectionFactorFlag.Name),
		SolcPath:                ctx.GlobalString(utils.SolcPathFlag.Name),
		PowFake:		 true,
	}

	if err := stack.Register(func(nsc *node.ServiceContext) (node.Service, error) {
		return NewBackend(nsc, ethConf, rpcclient.NewChannelClient(cl), cch)
	}); err != nil {
		utils.Fatalf("Failed to register the TMSP application service: %v", err)
	}
	*/

	stack, cfg := gethmain.MakeConfigNode(ctx, chainId)
	//utils.RegisterEthService(stack, &cfg.Eth)
	registerEthService(stack, &cfg.Eth, ctx, pNode, cch)

	if ctx.GlobalBool(utils.DashboardEnabledFlag.Name) {
		utils.RegisterDashboardService(stack, &cfg.Dashboard, ""/*gitCommit*/)
	}
	// Whisper must be explicitly enabled by specifying at least 1 whisper flag or in dev mode
	shhEnabled := gethmain.EnableWhisper(ctx)
	shhAutoEnabled := !ctx.GlobalIsSet(utils.WhisperEnabledFlag.Name) && ctx.GlobalIsSet(utils.DeveloperFlag.Name)
	if shhEnabled || shhAutoEnabled {
		if ctx.GlobalIsSet(utils.WhisperMaxMessageSizeFlag.Name) {
			cfg.Shh.MaxMessageSize = uint32(ctx.Int(utils.WhisperMaxMessageSizeFlag.Name))
		}
		if ctx.GlobalIsSet(utils.WhisperMinPOWFlag.Name) {
			cfg.Shh.MinimumAcceptedPOW = ctx.Float64(utils.WhisperMinPOWFlag.Name)
		}
		utils.RegisterShhService(stack, &cfg.Shh)
	}

	// Add the Ethereum Stats daemon if requested.
	if cfg.Ethstats.URL != "" {
		utils.RegisterEthStatsService(stack, cfg.Ethstats.URL)
	}

	stack.GatherServices()

	return stack
}


// registerEthService adds an Ethereum client to the stack.
func registerEthService(stack *node.Node, cfg *eth.Config, cliCtx *cli.Context,
                        pNode tendermint.PChainP2P, cch core.CrossChainHelper) {
	var err error
	if cfg.SyncMode == downloader.LightSync {
		err = stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
			return les.New(ctx, cfg, nil, nil)
		})
	} else {
		err = stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
			//return NewBackend(ctx, cfg, cliCtx, pNode, cch)
			fullNode, err := eth.New(ctx, cfg, cliCtx, pNode, cch)
			if fullNode != nil && cfg.LightServ > 0 {
				ls, _ := les.NewLesServer(fullNode, cfg)
				fullNode.AddLesServer(ls)
			}
			return fullNode, err
		})
	}
	if err != nil {
		utils.Fatalf("Failed to register the Ethereum service: %v", err)
	}
}
