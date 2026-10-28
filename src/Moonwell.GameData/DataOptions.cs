namespace Moonwell.GameData;

/// <summary>A map's game data set, as World Editor words it (<c>Melee</c>).</summary>
public enum DataSetOption
{
    /// <summary>World Editor's default set.</summary>
    Default,

    /// <summary>The custom set.</summary>
    Custom,

    /// <summary>The melee set.</summary>
    Melee,
}

/// <summary>A map's game data version, as World Editor words it (<c>FrozenThrone</c>).</summary>
public enum DataVersionOption
{
    /// <summary>Warcraft III: Reign of Chaos.</summary>
    ReignOfChaos,

    /// <summary>Warcraft III: The Frozen Throne.</summary>
    FrozenThrone,

    /// <summary>Forsaken Kingdom, the game's base data.</summary>
    ForsakenKingdom,
}

/// <summary>The tables the game reads for a map's game data option.</summary>
/// <param name="Values">The tables the game takes values from (<c>custom_v1</c>); <c>base</c> for the base tables.</param>
/// <param name="Objects">The tables that decide whether a standard object exists (<c>custom_v1</c>); <c>base</c> for the base tables.</param>
public sealed record DataTables(string Values, string Objects);

/// <summary>Which tables the game reads for a map's data set and data version (measured in the game; see docs/game-facts.md).</summary>
public static class DataOptions
{
    private const string Base = "base";

    /// <summary>The tables the game reads for the pair.</summary>
    /// <param name="set">The map's game data set.</param>
    /// <param name="version">The map's game data version.</param>
    /// <returns>For example <c>base</c> for the values and <c>custom_v1</c> for the objects under Melee / Frozen Throne.</returns>
    public static DataTables TablesOf(DataSetOption set, DataVersionOption version)
    {
        if (version == DataVersionOption.ForsakenKingdom)
        {
            return new DataTables(Base, Base);
        }

        if (set == DataSetOption.Melee)
        {
            // Under Melee / Frozen Throne the game takes every value from the base tables but has only the objects custom_v1 has
            // (inferred from three units).
            return version == DataVersionOption.ReignOfChaos
                ? new DataTables("melee_v0", "melee_v0")
                : new DataTables(Base, "custom_v1");
        }

        var tables = version == DataVersionOption.ReignOfChaos ? "custom_v0" : "custom_v1";
        return new DataTables(tables, tables);
    }
}
