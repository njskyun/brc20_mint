前提条件，需要有fb全节点，不需要安装ord索引器，打的时候自己盯着进度
这套同样也适合btc（需要修改下main.go文件中的几处请求fb rpc的地方）

1.安装 Go1.23.1

wget https://golang.org/dl/go1.23.1.linux-amd64.tar.gz

sudo tar -C /usr/local -xzf go1.23.1.linux-amd64.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

source ~/.bashrc

go version

2. clone代码库
   
git clone https://github.com/njskyun/brc20_mint.git

3.安装依赖

go mod tidy

4.打开main.go，这个文件可随意根据自己情况修改，这只是简易版本，复杂功能可这里自己实现

修改必要参数 ：

gas_fee, 当填写0时候，会自动采取当前链上gas

ordinals, 要打铭文内容

minterPrikey, 负责打铭文的地址私钥

myaddr 铭文接收地址

5.运行

go run .
