# AVX-512 保証ベンチマークVM (AWS c7i = Sapphire Rapids)
#
# 使い方:
#   cd infra && terraform init && terraform apply
#   cd .. && make remote-bench
#   使い終わったら: cd infra && terraform destroy

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
  }
}

provider "aws" {
  region = var.region
}

# SSH を現在のグローバルIPだけに開ける
data "http" "myip" {
  url = "https://checkip.amazonaws.com"
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# 専用ミニ VPC(デフォルト VPC が無いアカウントでも自己完結で動くように)
resource "aws_vpc" "bench" {
  cidr_block = "10.99.0.0/24"

  tags = {
    Name    = "simd-search-bench"
    Project = "gocon2026-simd-search"
  }
}

resource "aws_subnet" "bench" {
  vpc_id                  = aws_vpc.bench.id
  cidr_block              = "10.99.0.0/25"
  map_public_ip_on_launch = true

  tags = {
    Name = "simd-search-bench"
  }
}

resource "aws_internet_gateway" "bench" {
  vpc_id = aws_vpc.bench.id

  tags = {
    Name = "simd-search-bench"
  }
}

resource "aws_route_table" "bench" {
  vpc_id = aws_vpc.bench.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.bench.id
  }

  tags = {
    Name = "simd-search-bench"
  }
}

resource "aws_route_table_association" "bench" {
  subnet_id      = aws_subnet.bench.id
  route_table_id = aws_route_table.bench.id
}

resource "aws_key_pair" "bench" {
  key_name   = "simd-search-bench"
  public_key = file(pathexpand(var.ssh_public_key_path))
}

resource "aws_security_group" "bench" {
  name_prefix = "simd-search-bench-"
  description = "SSH from my IP only"
  vpc_id      = aws_vpc.bench.id

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${chomp(data.http.myip.response_body)}/32"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "bench" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.bench.key_name
  subnet_id              = aws_subnet.bench.id
  vpc_security_group_ids = [aws_security_group.bench.id]
  user_data              = file("${path.module}/user_data.sh")

  root_block_device {
    volume_size = 16
    volume_type = "gp3"
  }

  tags = {
    Name    = "simd-search-bench"
    Project = "gocon2026-simd-search"
  }
}
