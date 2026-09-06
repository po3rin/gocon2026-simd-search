variable "region" {
  description = "AWS region"
  type        = string
  default     = "ap-northeast-1"
}

variable "instance_type" {
  description = "AVX-512 を持つ Sapphire Rapids 世代を選ぶこと (c7i / m7i / r7i)"
  type        = string
  default     = "c7i.large"
}

variable "ssh_public_key_path" {
  description = "VM に登録する SSH 公開鍵"
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}
