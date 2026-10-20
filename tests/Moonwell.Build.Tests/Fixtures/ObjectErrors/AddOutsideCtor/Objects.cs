using Moonwell;

namespace AddOutsideCtor
{
    public sealed class AddOutsideCtorMap : Map
    {
    }

    public class Grow : Moonwell.Objects.Human.Units.Footman
    {
        public void Learn()
        {
            Abilities.Add<Moonwell.Objects.Human.Abilities.Defend>();
        }
    }
}
