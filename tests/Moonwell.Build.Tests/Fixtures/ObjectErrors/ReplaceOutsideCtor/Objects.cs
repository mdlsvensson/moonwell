using Moonwell;

namespace ReplaceOutsideCtor
{
    public sealed class ReplaceOutsideCtorMap : Map
    {
    }

    public class Grow : Moonwell.Objects.Human.Units.Footman
    {
        public void Learn()
        {
            Abilities.Replace<Moonwell.Objects.Human.Abilities.Defend, Moonwell.Objects.Human.Abilities.Avatar>();
        }
    }
}
