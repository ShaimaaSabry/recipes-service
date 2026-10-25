output "bucket_name" {
  description = "Name of the photos bucket."
  value       = aws_s3_bucket.photos.bucket
}

output "photos_url" {
  description = "CloudFront base URL (https://...) photos are served from."
  value       = "https://${aws_cloudfront_distribution.photos.domain_name}"
}
