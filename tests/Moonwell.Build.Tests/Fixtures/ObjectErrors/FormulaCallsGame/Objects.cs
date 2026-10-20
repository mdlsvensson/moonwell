using Moonwell;

namespace FormulaCallsGame
{
    public sealed class FormulaCallsGameMap : Map
    {
    }

    public class Bolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Bolt()
        {
            Tooltip = PerLevel(level => "Deals " + Natives.GetRandomInt(1, 2));
        }
    }
}
