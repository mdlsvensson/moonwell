namespace Moonwell
{
    /// <summary>The sound a hit makes, which the game calls its weapon type, numbered as the game numbers them. A missile's hit has none.</summary>
    public enum WeaponSound
    {
        /// <summary>No sound: missiles and most code damage.</summary>
        None = 0,

        /// <summary>Metal, light chop.</summary>
        MetalLightChop = 1,

        /// <summary>Metal, medium chop.</summary>
        MetalMediumChop = 2,

        /// <summary>Metal, heavy chop.</summary>
        MetalHeavyChop = 3,

        /// <summary>Metal, light slice.</summary>
        MetalLightSlice = 4,

        /// <summary>Metal, medium slice: a footman's.</summary>
        MetalMediumSlice = 5,

        /// <summary>Metal, heavy slice.</summary>
        MetalHeavySlice = 6,

        /// <summary>Metal, medium bash.</summary>
        MetalMediumBash = 7,

        /// <summary>Metal, heavy bash.</summary>
        MetalHeavyBash = 8,

        /// <summary>Metal, medium stab.</summary>
        MetalMediumStab = 9,

        /// <summary>Metal, heavy stab.</summary>
        MetalHeavyStab = 10,

        /// <summary>Wood, light slice.</summary>
        WoodLightSlice = 11,

        /// <summary>Wood, medium slice.</summary>
        WoodMediumSlice = 12,

        /// <summary>Wood, heavy slice.</summary>
        WoodHeavySlice = 13,

        /// <summary>Wood, light bash.</summary>
        WoodLightBash = 14,

        /// <summary>Wood, medium bash.</summary>
        WoodMediumBash = 15,

        /// <summary>Wood, heavy bash.</summary>
        WoodHeavyBash = 16,

        /// <summary>Wood, light stab.</summary>
        WoodLightStab = 17,

        /// <summary>Wood, medium stab.</summary>
        WoodMediumStab = 18,

        /// <summary>Claw, light slice.</summary>
        ClawLightSlice = 19,

        /// <summary>Claw, medium slice.</summary>
        ClawMediumSlice = 20,

        /// <summary>Claw, heavy slice.</summary>
        ClawHeavySlice = 21,

        /// <summary>Axe, medium chop.</summary>
        AxeMediumChop = 22,

        /// <summary>Rock, heavy bash.</summary>
        RockHeavyBash = 23,
    }
}
