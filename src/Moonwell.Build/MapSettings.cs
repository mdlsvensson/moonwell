namespace Moonwell.Build
{
    /// <summary>What the map's <c>Moonwell.Map</c> class declares, read on .NET.</summary>
    /// <param name="TypeName">The full name of the map class, usable after <c>global::</c> in generated C#.</param>
    /// <param name="Name">The map's name; empty keeps World Editor's value.</param>
    /// <param name="Author">The map's author; empty keeps World Editor's value.</param>
    /// <param name="Description">The map's description; empty keeps World Editor's value.</param>
    /// <param name="RecommendedPlayers">The recommended players; empty keeps World Editor's value.</param>
    /// <param name="DataSet">The map's <c>Moonwell.DataSet</c> as its number: 0 Default, 1 Custom, 2 Melee.</param>
    /// <param name="DataVersion">The map's <c>Moonwell.DataVersion</c> as its number: 0 ReignOfChaos, 1 FrozenThrone, 2 ForsakenKingdom.</param>
    public sealed record MapSettings(string TypeName, string Name, string Author, string Description, string RecommendedPlayers, int DataSet = 0, int DataVersion = 2);
}
