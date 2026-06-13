output "public_ip" {
  value = aws_instance.bench.public_ip
}

output "ssh_command" {
  value = "ssh ubuntu@${aws_instance.bench.public_ip}"
}
