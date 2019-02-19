package params


//Abnormal Behavior Test Cases
const (

	//keep normal behavior
	ABTC_NormalBehavior    uint64 = 0

	//always send block proposal even this node is not proposer
	ABTC_SendProposalBlock uint64 = 1

	//always vote nil for the prevote round
	ABTC_VoteNilForPrevote uint64 = 2

	//always vote nil for the precommit round
	ABTC_VoteNilForPrecommit uint64 = 3

	//add tx to transfer myself money when this node is proposer
	ABTC_AddInvalidTx uint64 = 5
)

var (
	CurrentABTestCase uint64 = 0   //by default, there is no abnormal behavior test case
)
