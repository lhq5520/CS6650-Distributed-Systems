Part II: ECR/ECS/Fargate Deployment

In this assignment, I deployed a containerized Go web service to AWS using
ECR, ECS, and Fargate.

Steps completed:

1. Loaded a pre-built Docker image (docker-album.tar) containing a Go web
   service with /albums and /albums/:id endpoints
2. Pushed the image to Amazon ECR (Elastic Container Registry)
3. Created an ECS cluster using Fargate (serverless)
4. Created a Task Definition specifying 0.25 vCPU and 0.5 GB memory
5. Ran the task with public IP enabled
6. Successfully tested the service via curl

Challenges:

- Understanding the difference between ECR (registry) and ECS (orchestration)
- Configuring security groups to allow port 8080
- Getting the public IP from the network interface

What I learned:

- ECR/ECS workflow is much more scalable than manually deploying to EC2
- Fargate eliminates the need to manage servers
- Container orchestration makes deployment repeatable and consistent
- The importance of IAM roles (LabRole) for ECS tasks
