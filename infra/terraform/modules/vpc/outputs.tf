output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value       = aws_subnet.private[*].id
  description = "Subnets for EKS nodes and the controle plane, order matches the AZ order."
}

output "vpc_cidr_block" {
  value = aws_vpc.this.cidr_block
}

output "nat_public_ip" {
  value       = aws_eip.nat.public_ip
  description = "This will change on every rebuild, if torn down you will probably have to update an IP whitelist somewhere after it comes back up."
}