scp -i ~/aws_learner.pem docker-album.tar ec2-user@ec2-44-249-250-141.us-west-2.compute.amazonaws.com:docker-app

ssh -i ~/aws_learner.pem ec2-user@ec2-54-184-12-146.us-west-2.compute.amazonaws.com

docker load -i docker-album.tar

docker run -d -p 8080:8080 docker-go-album:multistage
