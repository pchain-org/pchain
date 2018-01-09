为了保证目录的简介性，当前看到的文件结构并非实际使用开发和使用时的目录结构。

在开发和部署时，从github上下载完成以后，需要进行如下调整：

1，在当前目录下创建
.\src\github.com\tendermint\ethermint\，
将除了 !!restore!!, .git, !!readmefirst!!.txt 这3个目录和文件以外的所有文件都移至新创建的目录下

2，将 !!restore!!目录下的所有内容拷贝到当前目录

完成以上步骤以后，新的目录结构将可以进行IntelliJ Idea的项目展开，以及在Ubuntu14.0.4下的编译部署。
