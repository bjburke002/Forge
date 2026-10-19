output "vpc_id" {
  value = aws_vpc.forge_vpc.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "vpc_cidr_block" {
  value = aws_vpc.forge_vpc.cidr_block
}

output "nat_public_ip" {
  value = aws_eip.nat.public_ip
}