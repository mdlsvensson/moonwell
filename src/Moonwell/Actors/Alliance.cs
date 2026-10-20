using System;

namespace Moonwell
{
    /// <summary>What one player gives another, as the game's alliance types (ALLIANCE_*); it goes one way.</summary>
    [Flags]
    public enum Alliance
    {
        /// <summary>Nothing: the player is an enemy.</summary>
        None = 0,

        /// <summary>The player's units do not attack the other's.</summary>
        Passive = 1 << 0,

        /// <summary>The player's units call the other's for help.</summary>
        HelpRequest = 1 << 1,

        /// <summary>The player's units answer the other's calls for help.</summary>
        HelpResponse = 1 << 2,

        /// <summary>The player shares experience with the other.</summary>
        SharedXp = 1 << 3,

        /// <summary>The player's spells treat the other's units as allies.</summary>
        SharedSpells = 1 << 4,

        /// <summary>The player shares vision with the other.</summary>
        SharedVision = 1 << 5,

        /// <summary>The other player can control the player's units.</summary>
        SharedControl = 1 << 6,

        /// <summary>The other player can also control the player's buildings and train with them.</summary>
        SharedAdvancedControl = 1 << 7,

        /// <summary>The other player can rescue the player's units.</summary>
        Rescuable = 1 << 8,

        /// <summary>The player shares vision with the other, and the other cannot turn it off.</summary>
        SharedVisionForced = 1 << 9,

        /// <summary>blizzard.j's "allied": passive, both kinds of help, shared experience and shared spells.</summary>
        Allied = Passive | HelpRequest | HelpResponse | SharedXp | SharedSpells,
    }
}
