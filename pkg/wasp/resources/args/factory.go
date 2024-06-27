package args

// FactoryArgs contains the required parameters to generate all namespaced resources
type FactoryArgs struct {
	MaxAverageSwapInPerSecond  string `required:"true" split_words:"true"`
	MemoryAvailableThreshold   string `required:"true" split_words:"true"`
	MaxAverageSwapOutPerSecond string `required:"true" split_words:"true"`
	OperatorVersion            string `required:"true" split_words:"true"`
	WaspImage                  string `required:"true" split_words:"true"`
	DeployClusterResources     string `required:"true" split_words:"true"`
	Verbosity                  string `required:"true"`
	MinTimeInterval            string `required:"true"`
	PullPolicy                 string `required:"true" split_words:"true"`
	Namespace                  string
}
