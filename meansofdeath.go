package main

import (
	"errors"
	"regexp"
)

// represents a frag
type Death struct {
	Murderer *Player
	Victim   *Player
	Means    int
	Solo     bool // self-frag
}

// All possible means of death
const (
	ModUnknown = iota
	ModBlaster
	ModShotgun
	ModSShotgun
	ModMachinegun
	ModChaingun
	ModGrenade
	ModGSplash
	ModRocket
	ModRSplash
	ModHyperblaster
	ModRailgun
	ModBFGLaser
	ModBFGBlast
	ModBFGEffect
	ModHandgrenade // hit with grenade
	ModHGSplash
	ModWater
	ModSlime
	ModLava
	ModCrush
	ModTelefrag
	ModFalling
	ModSuicide
	ModHeldGrenade
	ModExplosive
	ModBarrel
	ModBomb
	ModExit
	ModSplash
	ModTargetLaser
	ModTriggerHurt
	ModHit
	ModTargetBlaster
	ModFriendlyFire
)

// figure out who killed who and how
func (cl *Client) CalculateDeath(obit string) (*Death, error) {
	death := &Death{}

	type ObitTest struct {
		matchstr string
		mod      int
	}

	// only has a victim
	solo := []ObitTest{
		{
			matchstr: "(.+) suicides",
			mod:      ModSuicide,
		},
		{
			matchstr: "(.+) cratered",
			mod:      ModFalling,
		},
		{
			matchstr: "(.+) was squished",
			mod:      ModCrush,
		},
		{
			matchstr: "(.+) sank like a rock",
			mod:      ModWater,
		},
		{
			matchstr: "(.+) melted",
			mod:      ModSlime,
		},
		{
			matchstr: "(.+) does a back flip into the lava",
			mod:      ModLava,
		},
		{
			matchstr: "(.+) blew up",
			mod:      ModExplosive,
		},
		{
			matchstr: "(.+) found a way out",
			mod:      ModExit,
		},
		{
			matchstr: "(.+) saw the light",
			mod:      ModTargetLaser,
		},
		{
			matchstr: "(.+) got blasted",
			mod:      ModTargetBlaster,
		},
		{
			matchstr: "(.+) was in the wrong place",
			mod:      ModSplash,
		},
		{
			matchstr: "(.+) tried to put the pin back in",
			mod:      ModHeldGrenade,
		},
		{
			matchstr: "(.+) tripped on .+ own grenade",
			mod:      ModGSplash,
		},
		{
			matchstr: "(.+) blew .+self up",
			mod:      ModRSplash,
		},
		{
			matchstr: "(.+) should have used a smaller gun",
			mod:      ModBFGBlast,
		},
		{
			matchstr: "(.+) killed .+self",
			mod:      ModSuicide,
		},
		{
			matchstr: "(.+) died",
			mod:      ModUnknown,
		},
	}

	// has a victim and an attacker
	duo := []ObitTest{
		{
			matchstr: "(.+) was blasted by (.+)",
			mod:      ModBlaster,
		},
		{
			matchstr: "(.+) was gunned down by (.+)",
			mod:      ModShotgun,
		},
		{
			matchstr: "(.+) was blown away by (.+)'s super shotgun",
			mod:      ModSShotgun,
		},
		{
			matchstr: "(.+) was machinegunned by (.+)",
			mod:      ModMachinegun,
		},
		{
			matchstr: "(.+) was cut in half by (.+)'s chaingun",
			mod:      ModChaingun,
		},
		{
			matchstr: "(.+) was popped by (.+)'s grenade",
			mod:      ModGrenade,
		},
		{
			matchstr: "(.+) was shredded by (.+)'s shrapnel",
			mod:      ModGSplash,
		},
		{
			matchstr: "(.+) ate (.+)'s rocket",
			mod:      ModRocket,
		},
		{
			matchstr: "(.+) almost dodged (.+)'s rocket",
			mod:      ModRSplash,
		},
		{
			matchstr: "(.+) was melted by (.+)'s hyperblaster",
			mod:      ModHyperblaster,
		},
		{
			matchstr: "(.+) was railed by (.+)",
			mod:      ModRailgun,
		},
		{
			matchstr: "(.+) saw the pretty lights from (.+)'s BFG",
			mod:      ModBFGLaser,
		},
		{
			matchstr: "(.+) was disintegrated by (.+)'s BFG blast",
			mod:      ModBFGBlast,
		},
		{
			matchstr: "(.+) couldn't hide from (.+)'s BFG",
			mod:      ModBFGEffect,
		},
		{
			matchstr: "(.+) caught (.+)'s handgrenade",
			mod:      ModHandgrenade,
		},
		{
			matchstr: "(.+) didn't see (.+)'s handgrenade",
			mod:      ModHGSplash,
		},
		{
			matchstr: "(.+) feels (.+)'s pain",
			mod:      ModHeldGrenade,
		},
		{
			matchstr: "(.+) tried to invade (.+)'s personal space",
			mod:      ModTelefrag,
		},
	}

	// frags involving 2 people are more common, do them first
	for _, frag := range duo {
		pattern, err := regexp.Compile(frag.matchstr)
		if err != nil {
			continue
		}

		if pattern.Match([]byte(obit)) {
			submatches := pattern.FindAllStringSubmatch(obit, -1)
			death.Means = frag.mod
			death.Victim = cl.FindPlayerByName(submatches[0][1])
			death.Murderer = cl.FindPlayerByName(submatches[0][2])
			death.Solo = false
			return death, nil
		}
	}

	// frags involving 1 person
	for _, frag := range solo {
		pattern, err := regexp.Compile(frag.matchstr)
		if err != nil {
			continue
		}

		if pattern.Match([]byte(obit)) {
			submatches := pattern.FindAllStringSubmatch(obit, -1)
			death.Means = frag.mod
			death.Victim = cl.FindPlayerByName(submatches[0][1])
			death.Murderer = nil
			death.Solo = true
			return death, nil
		}
	}

	return death, errors.New("obituary not recognised")
}
