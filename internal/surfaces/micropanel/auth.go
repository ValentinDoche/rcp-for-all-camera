package micropanel

// The panel only streams input reports after a challenge/response handshake on
// feature report 0x05. The algorithm is the one used by Blackmagic's other USB
// panels (DaVinci Speed Editor), publicly documented by the community
// (https://github.com/smunaut/blackmagic-misc).

var authEven = [8]uint64{
	0x3ae1206f97c10bc8, 0x2a9ab32bebf244c6, 0x20a6f8b8df9adf0a, 0xaf80ece52cfc1719,
	0xec2ee2f7414fd151, 0xb055adfd73344a15, 0xa63d2e3059001187, 0x751bf623f42e0dde,
}

var authOdd = [8]uint64{
	0x3e22b34f502e7fde, 0x24656b981875ab1c, 0xa17f3456df7bf8c3, 0x6df72e1941aef698,
	0x72226f011e66ab94, 0x3831a3c606296b42, 0xfd7ff81881332c89, 0x61a3f6474ff236c6,
}

const authMask = 0xa79a63f585d37bf0

func rol8(v uint64) uint64 { return v<<56 | v>>8 }

// authResponse computes the 64-bit answer to a panel challenge.
func authResponse(challenge uint64) uint64 {
	n := challenge & 7
	v := challenge
	for i := uint64(0); i < n; i++ {
		v = rol8(v)
	}
	var k uint64
	if v&1 == (0x78>>n)&1 {
		k = authEven[n]
	} else {
		v ^= rol8(v)
		k = authOdd[n]
	}
	return v ^ (rol8(v) & authMask) ^ k
}
