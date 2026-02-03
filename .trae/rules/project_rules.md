1. docker build的时候注意：docker镜像：nginx:latest，golang:1.24-alpine，alpine:3.19，mysql:latest 已经存在本地，请不要去网上拉取！！！
2. 启动容器前先查一下容器是否已经存在，存在则先删除
3. 本项目会启动3个容器：ghh-server(后端服务)，ghh-frontend(前端服务)，mysql-ghh(数据库服务)
4. 如果前端代码发生变更，需要先在本地编译再构建前端镜像
5. 每次测试请按照部署脚本来部署：deploy.ps1