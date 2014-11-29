# lxcify

lxcify installs desktop applications into unprivileged LXC containers. Access
to host devices is allowed but only as mapped in an application's configuration
file. Host devices can also be replaced with surrogate devices for interesting
results.

This isn't perfect security, but lxcify can be a useful tool for improving
security, privacy and optionality. Especially when faced with using software
in which you don't have much of a choice (work requires it, etc.)

Possible uses of lxcify:

## Limited sandboxing of non-free software

Non-free software can be given access to host devices for the duration of their
execution. While non-free software _could_ be using these devices for devious
purposes, you can be sure that they are not using them when the container is
not running! lxcify also prevents non-free software from modifying your host system.
This means no cleanup files, potentially installed background processes, access
to normal files and services running on the host.

## Poorly-packaged software

Poorly packaged software which tends to muck up a host can be safely run in a
container. No more worrying about fragments and files left behind when trying
to "uninstall" it. Just lxc-destroy when you're done.

## Multiple contexts

Isolating your "usage contexts" into separate browsers can improve privacy and
security. For example, it can be quite sensible to run separate browsers for:

* Work
* Entertainment (Flash-enabled)
* Banking
* Online purchasing
* Facebook

Why should your personal browsing reveal your spending habits, or potentially
compromise your bank account?

# Credits

lxcify is inspired by, and based on Stéphane Graber's blog post, [LXC 1.0: GUI in containers](https://www.stgraber.org/2014/02/09/lxc-1-0-gui-in-containers/). lxcify uses [go-lxc](https://gopkg.in/lxc/go-lxc.v1) for creating and manipulating LXC containers.

---

Copyright (c) 2014 Casey Marshall
