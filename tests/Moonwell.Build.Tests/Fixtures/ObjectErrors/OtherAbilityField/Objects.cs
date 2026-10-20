using Moonwell;

namespace OtherAbilityField
{
    public sealed class OtherAbilityFieldMap : Map
    {
    }

    public class Bolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Bolt()
        {
            Set("Hhb1", 1, 50f);
        }
    }
}
