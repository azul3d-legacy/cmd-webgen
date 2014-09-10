# Versioning

Azul3D packages are versioned such that when API compatible changes are made, programs using the previous package API are not broken. This document describes the versioning scheme used by Azul3D for all package imports.

* [The Basics](#the-basics)
* [Development Versions](#development-versions)
* [Implementation - Git Tags](#implementation-git-tags)
* [Implementation - Google App Engine](#implementation-google-app-engine)

# The Basics

In the most basic form a package import path has the following syntax:</p>

```
azul3d.org/somepkg.v(major)[.(minor)]
```

For instance the import path for *somepkg* whose major version is one would be:

```
azul3d.org/somepkg.v1
```

Or the import path for *somepkg* whose major version is one and minor version is two:

```
azul3d.org/somepkg.v1.2
```

Even though the import path ends with a version number, the package is still referenced in code the same exact way:

```
somepkg.DoSomething()
```

This is because Go analyzes the package name from the source code files -- not the import URL.

# Development Versions

There is a special path extension, *.dev*, which signifies the in-development version of the package, like so:

```
azul3d.org/somepkg.dev
```

This special extension should only be used if you need features only found in the in-development versions of packages, but most of the time you should never use it and instead stick with the most-recently released version (mentioned on the documentation page for that package).

# Implementation - Git Tags

In our implementation of versioning, we use git tags like "v1" or "v1.2" to reference a specifically released version. This is important for a few reasons:

* Tags can be moved to newer revisions, so that fixes can still be made to versions post-release (as long as no backwards-incompatible changes have been made yet).
* Tags are easy to view through the GitHub API which makes writing versioned-documentation easy.
* Version tags can be viewed through the GitHub's website (which even allows downloading zipped archives of version releases).

# Implementation - Google App Engine

The Go tool is not aware of tags as version numbers, this means that we must theoretically host a different repository for each version of each package. But in reality we can actually work around this by using some trickery:

Using remote import path HTML meta tags (see [here](http://golang.org/cmd/go/#hdr-Remote_import_paths)) the Go tool will let us alias an GitHub repository under a different domain name, such that the import path:

```
github.com/azul3d/somepkg
```

Becomes:

```
azul3d.org/somepkg
```

Then we make use of an appengine application responsible for acting as a proxy of sorts (view the source of this application [here](https://github.com/azul3d/appengine)). For standard web requests, it simply makes a proxied request to our `azul3d.github.io` domain, where the entire website content generated via the [webgen tool](https://github.com/azul3d/cmd-webgen) is hosted. In practice this is not slow because the website is fronted by [CloudFlare](https://www.cloudflare.com/) which caches these requests.

When the appengine application receives a request from the go-get tool (signified by a URL query term `?go-get=1`), it first makes contact to GitHub to see if the repository exists, if it does then it serves a meta tag to go-get like:

```
<meta name="go-import" content="azul3d.org/somepkg.v1 git azul3d.org/somepkg.v1/repo">
```

Then go get wil run the git clone command like:

```
git clone azul3d.org/somepkg.v1/repo $GOPATH/src/azul3d.org/somepkg.v1
```

And since we claimed in the meta tag that `azul3d.org/somepkg.v1/repo` was our git repository, git clone issues a few HTTP requests to clone the repository hosted there. We intercept those requests and forward them over to GitHub, while also pointing them towards the correct version tag specified in the URL.

