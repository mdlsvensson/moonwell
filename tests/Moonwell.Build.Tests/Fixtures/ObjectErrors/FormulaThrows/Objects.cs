using Moonwell;

namespace FormulaThrows
{
    public sealed class FormulaThrowsMap : Map
    {
    }

    public class Bolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Bolt()
        {
            Tooltip = PerLevel<string>(level => throw new System.InvalidOperationException("no level " + level));
        }
    }
}
