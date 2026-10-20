using Moonwell;

namespace SetByIdOutsideCtor
{
    public sealed class SetByIdOutsideCtorMap : Map
    {
    }

    public class Grow : Moonwell.Objects.Human.Units.Footman
    {
        public void Retune()
        {
            Set("uhpm", 5);
        }
    }
}
