# 容器部署模式

docker run -d --name kubez-ansible --net host -v /root/.ssh/:/root/.ssh/ -v /etc:/etc -w /root --restart=always pixiuio/kubez-ansible:v3.0.1 sleep infinity

docker run -d --name kubez-ansible-bootstrap-servers -e COMMAND=bootstrap-servers  -v /etc/kubez:/configs jacky06/kubez-ansible:v3.0.1
docker run -d --name kubez-ansible-deploy -e COMMAND=deploy -v /etc/kubez:/configs jacky06/kubez-ansible:v3.0.1
