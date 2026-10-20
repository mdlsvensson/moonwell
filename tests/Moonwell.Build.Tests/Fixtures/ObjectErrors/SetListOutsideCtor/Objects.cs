using Moonwell;

namespace SetListOutsideCtor
{
    public sealed class SetListOutsideCtorMap : Map
    {
    }

    public class Grow : Moonwell.Objects.Human.Units.Footman
    {
        public void Learn()
        {
            Abilities.Set<Moonwell.Objects.Human.Abilities.Defend>();
        }
    }
}
