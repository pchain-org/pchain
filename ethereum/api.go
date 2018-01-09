package ethereum

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/node"
	"math/big"

	"github.com/syndtr/goleveldb/leveldb/errors"
	//"github.com\ethereum\go-ethereum\eth\api.go
	"reflect"
	"fmt"
)

type ethApis struct{
	loaded uint
	ethBackend *Backend
	//n *node.Node
	pubEthApi *eth.PublicEthereumAPI
}

var EthApi = ethApis {
	ethBackend : nil,
	pubEthApi : nil,
}

func ReloadEthApi(n *node.Node, backend *Backend) {

	fmt.Printf("ReloadEthApi() 0\n")

	EthApi.ethBackend = backend

	apis := n.RpcAPIs();

	pubEthApiType := reflect.TypeOf(&eth.PublicEthereumAPI{})
	str := pubEthApiType.String()
	pkg := pubEthApiType.PkgPath()

	for i := 0; i < len(apis); i++ {
		api := apis[i]

		//fmt.Println("----------------------")
		svrType := reflect.TypeOf(api.Service)
		//PrintType(svrType)

		if svrType.String() == str && svrType.PkgPath() == pkg {
			EthApi.pubEthApi = api.Service.(*eth.PublicEthereumAPI)
		}
	}

	//PrintType(reflect.TypeOf(&eth.PublicEthereumAPI{}))
}

func Coinbase() (common.Address, error) {

	if(EthApi.pubEthApi != nil) {
		return EthApi.pubEthApi.Coinbase()
	}

	return common.Address{}, errors.New("PublicEthereumAPI not included")
}

func GetBalance(account common.Address) (*big.Int, error) {

	if(EthApi.ethBackend == nil) {
		return nil, errors.New("EthApi not initialized")
	}

	ethereum := EthApi.ethBackend.Ethereum()
	bc := ethereum.BlockChain()
	state, err := bc.State()

	if (err != nil) {
		return nil, errors.New("EthApi get state failed")
	}

	return state.GetBalance(account), nil
}

func PrintType(t reflect.Type) {
	fmt.Println("--------------------")
	// ----- 通用方法 -----
	fmt.Println("String             :", t.String())     // 类型字符串
	fmt.Println("Name               :", t.Name())       // 类型名称
	fmt.Println("PkgPath            :", t.PkgPath())    // 所在包名称
	fmt.Println("Kind               :", t.Kind())       // 所属分类
	fmt.Println("Size               :", t.Size())       // 内存大小
	fmt.Println("Align              :", t.Align())      // 字节对齐
	fmt.Println("FieldAlign         :", t.FieldAlign()) // 字段对齐
	fmt.Println("NumMethod          :", t.NumMethod())  // 方法数量
	if t.NumMethod() > 0 {
		i := 0
		for ; i < t.NumMethod()-1; i++ {
			fmt.Println("    ┣", t.Method(i).Name) // 通过索引定位方法
		}
		fmt.Println("    ┗", t.Method(i).Name) // 通过索引定位方法
	}
	if sm, ok := t.MethodByName("String"); ok { // 通过名称定位方法
		fmt.Println("MethodByName       :", sm.Index, sm.Name)
	}
	//fmt.Println("Implements(*eth.PublicEthereumAPI)    :", t.Implements(reflect.TypeOf(&eth.PublicEthereumAPI{})))  // 是否实现了指定接口
	//fmt.Println("ConvertibleTo(*eth.PublicEthereumAPI) :", t.ConvertibleTo(reflect.TypeOf(&eth.PublicEthereumAPI{}))) // 是否可转换为指定类型
	//fmt.Println("AssignableTo(*eth.PublicEthereumAPI)  :", t.AssignableTo(reflect.TypeOf(&eth.PublicEthereumAPI{})))  // 是否可赋值给指定类型的变量
	fmt.Println("Comparable         :", t.Comparable())               // 是否可进行比较操作
	// ----- 特殊类型 -----
	switch t.Kind() {
	// 指针型：
	case reflect.Ptr:
		fmt.Println("=== 指针型 ===")
		// 获取指针所指对象
		t = t.Elem()
		fmt.Printf("转换到指针所指对象 : %v\n", t.String())
		// 递归处理指针所指对象
		PrintType(t)
		return
	// 自由指针型：
	case reflect.UnsafePointer:
		fmt.Println("=== 自由指针 ===")
	// ...
	// 接口型：
	case reflect.Interface:
		fmt.Println("=== 接口型 ===")
	// ...
	}
	// ----- 简单类型 -----
	// 数值型：
	if reflect.Int <= t.Kind() && t.Kind() <= reflect.Complex128 {
		fmt.Println("=== 数值型 ===")
		fmt.Println("Bits               :", t.Bits()) // 位宽
	}
	// ----- 复杂类型 -----
	switch t.Kind() {
	// 数组型：
	case reflect.Array:
		fmt.Println("=== 数组型 ===")
		fmt.Println("Len                :", t.Len())  // 数组长度
		fmt.Println("Elem               :", t.Elem()) // 数组元素类型
	// 切片型：
	case reflect.Slice:
		fmt.Println("=== 切片型 ===")
		fmt.Println("Elem               :", t.Elem()) // 切片元素类型
	// 映射型：
	case reflect.Map:
		fmt.Println("=== 映射型 ===")
		fmt.Println("Key                :", t.Key())  // 映射键
		fmt.Println("Elem               :", t.Elem()) // 映射值类型
	// 通道型：
	case reflect.Chan:
		fmt.Println("=== 通道型 ===")
		fmt.Println("ChanDir            :", t.ChanDir()) // 通道方向
		fmt.Println("Elem               :", t.Elem())    // 通道元素类型
	// 结构体：
	case reflect.Struct:
		fmt.Println("=== 结构体 ===")
		fmt.Println("NumField           :", t.NumField()) // 字段数量
		if t.NumField() > 0 {
			var i, j int
			// 遍历结构体字段
			for i = 0; i < t.NumField()-1; i++ {
				field := t.Field(i) // 获取结构体字段
				fmt.Printf("    ├ %v\n", field.Name)
				// 遍历嵌套结构体字段
				if field.Type.Kind() == reflect.Struct && field.Type.NumField() > 0 {
					for j = 0; j < field.Type.NumField()-1; j++ {
						subfield := t.FieldByIndex([]int{i, j}) // 获取嵌套结构体字段
						fmt.Printf("    │    ├ %v\n", subfield.Name)
					}
					subfield := t.FieldByIndex([]int{i, j}) // 获取嵌套结构体字段
					fmt.Printf("    │    └ %%v\n", subfield.Name)
				}
			}
			field := t.Field(i) // 获取结构体字段
			fmt.Printf("    └ %v\n", field.Name)
			// 通过名称查找字段
			if field, ok := t.FieldByName("ptr"); ok {
				fmt.Println("FieldByName(ptr)   :", field.Name)
			}
			// 通过函数查找字段
			if field, ok := t.FieldByNameFunc(func(s string) bool { return len(s) > 3 }); ok {
				fmt.Println("FieldByNameFunc    :", field.Name)
			}
		}
	// 函数型：
	case reflect.Func:
		fmt.Println("=== 函数型 ===")
		fmt.Println("IsVariadic         :", t.IsVariadic()) // 是否具有变长参数
		fmt.Println("NumIn              :", t.NumIn())      // 参数数量
		if t.NumIn() > 0 {
			i := 0
			for ; i < t.NumIn()-1; i++ {
				fmt.Println("    ┣", t.In(i)) // 获取参数类型
			}
			fmt.Println("    ┗", t.In(i)) // 获取参数类型
		}
		fmt.Println("NumOut             :", t.NumOut()) // 返回值数量
		if t.NumOut() > 0 {
			i := 0
			for ; i < t.NumOut()-1; i++ {
				fmt.Println("    ┣", t.Out(i)) // 获取返回值类型
			}
			fmt.Println("    ┗", t.Out(i)) // 获取返回值类型
		}
	}
}