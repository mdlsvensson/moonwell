namespace Moonwell.Hosting
{
    /// <summary>The states of a player the runtime reads and sets: its resources, food and upkeep.</summary>
    public enum PlayerState
    {
        /// <summary>The gold the player has.</summary>
        Gold,

        /// <summary>The lumber the player has.</summary>
        Lumber,

        /// <summary>The food the player's units use.</summary>
        FoodUsed,

        /// <summary>The food the player's buildings give.</summary>
        FoodCap,

        /// <summary>The most food the player can have, whatever its buildings give.</summary>
        FoodCapCeiling,

        /// <summary>The gold the player has gathered in the game.</summary>
        GoldGathered,

        /// <summary>The lumber the player has gathered in the game.</summary>
        LumberGathered,

        /// <summary>The percent of gathered gold the player loses to upkeep.</summary>
        GoldUpkeepRate,

        /// <summary>The percent of gathered lumber the player loses to upkeep.</summary>
        LumberUpkeepRate,
    }
}
