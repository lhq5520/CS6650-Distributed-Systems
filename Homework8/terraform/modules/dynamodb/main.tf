# =============================================================================
# DynamoDB Module - HW8 STEP II
# Single table design for shopping cart
# =============================================================================

resource "aws_dynamodb_table" "shopping_carts" {
  name         = "${var.service_name}-shopping-carts"
  billing_mode = "PAY_PER_REQUEST"   # On-demand: no capacity planning needed
  hash_key     = "PK"                # Partition key
  range_key    = "SK"                # Sort key

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  tags = {
    Name = "${var.service_name}-shopping-carts"
    HW   = "8"
  }
}
