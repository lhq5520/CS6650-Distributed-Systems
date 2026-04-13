output "ec2_public_ip" {
  description = "Elastic IP of the app server — use this as base_url"
  value       = aws_eip.app.public_ip
}

output "rds_endpoint" {
  description = "RDS PostgreSQL endpoint"
  value       = aws_db_instance.postgres.address
}

output "s3_bucket_name" {
  description = "S3 bucket for photos"
  value       = aws_s3_bucket.photos.bucket
}

output "ssh_command" {
  description = "SSH into EC2"
  value       = "ssh -i <your-key>.pem ec2-user@${aws_eip.app.public_ip}"
}

output "submit_command" {
  description = "Submit to ChaosArena"
  value       = <<-EOT
    curl -X POST http://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \
      -H "Content-Type: application/json" \
      -d '{"email":"YOUR_EMAIL","nickname":"YOUR_NICK","base_url":"http://${aws_eip.app.public_ip}","contract":"v1-album-store"}'
  EOT
}

output "deploy_steps" {
  description = "Steps to deploy after terraform apply"
  value       = <<-EOT

    === Deploy Steps ===
    1. Cross-compile:
       GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

    2. Upload binary:
       scp -i <your-key>.pem server ec2-user@${aws_eip.app.public_ip}:/opt/album-store/server

    3. SSH in and start:
       ssh -i <your-key>.pem ec2-user@${aws_eip.app.public_ip}
       sudo chmod +x /opt/album-store/server
       sudo systemctl start album-store
       sudo systemctl status album-store

    4. Test health:
       curl http://${aws_eip.app.public_ip}/health

    5. Submit to ChaosArena (see submit_command output)
  EOT
}
