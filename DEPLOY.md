# Deploy to GitHub

Simple steps to deploy this project to GitHub.

## Step 1: Initialize Git (if not already done)

```bash
git init
```

## Step 2: Add All Files

```bash
git add .
```

## Step 3: Create Initial Commit

```bash
git commit -m "Initial commit: Browserbase Mini with complete documentation"
```

## Step 4: Create GitHub Repository

1. Go to https://github.com/new
2. Repository name: `browserbase-mini`
3. Description: "Lightweight browser automation service with Docker and Puppeteer"
4. Choose Public or Private
5. **DO NOT** initialize with README (you already have one)
6. Click "Create repository"

## Step 5: Add Remote and Push

```bash
# Add your GitHub repository as remote
git remote add origin https://github.com/YOUR_USERNAME/browserbase-mini.git

# Push to GitHub
git branch -M main
git push -u origin main
```

Replace `YOUR_USERNAME` with your actual GitHub username.

## That's It!

Your repository is now on GitHub at:
`https://github.com/YOUR_USERNAME/browserbase-mini`

## Future Updates

After making changes:

```bash
git add .
git commit -m "Your commit message"
git push
```
