package relay

import (
	electra2 "github.com/attestantio/go-eth2-client/api/v1/electra"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/go-boost-utils/ssz"
	"testing"
	"time"

	v1 "github.com/attestantio/go-builder-client/api/v1"
)

func TestValidatorRegistration(t *testing.T) {
	registration := &v1.SignedValidatorRegistration{
		Message: &v1.ValidatorRegistration{
			FeeRecipient: bellatrix.ExecutionAddress(common.HexToAddress("0xdb65fEd33dc262Fe09D9a2Ba8F80b329BA25f941")),
			Timestamp:    time.Unix(1606824043, 0),
			GasLimit:     30000000,
			Pubkey: phase0.BLSPubKey(common.FromHex(
				"0x84e975405f8691ad7118527ee9ee4ed2e4e8bae973f6e29aa9ca9ee4aea83605ae3536d22acc9aa1af0545064eacf82e",
			)),
		},
		Signature: phase0.BLSSignature(common.FromHex(
			"0xaf12df007a0c78abb5575067e5f8b089cfcc6227e4a91db7dd8cf517fe86fb944ead859f0781277d9b78c672e4a18c5d06368b603374673cf2007966cece9540f3a1b3f6f9e1bf421d779c4e8010368e6aac134649c7a009210780d401a778a5",
		)),
	}
	domainBuilder := ssz.ComputeDomain(ssz.DomainTypeAppBuilder, [4]byte{}, phase0.Root{})

	ok, err := ssz.VerifySignature(registration.Message, domainBuilder, registration.Message.Pubkey[:], registration.Signature[:])
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Failed to verify signature")
	}
}

func TestExecutionPayload(t *testing.T) {
	const signedBlock = `
{
  "signed_block": {
    "message": {
      "slot": "944793",
      "proposer_index": "257015",
      "parent_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
      "state_root": "0xe0478c4f2abc6c01b965a926929527a70f4f81022dc16205f8214d8e7fd027a8",
      "body": {
        "randao_reveal": "0xa8f58c4f23c7124195e7f33534c410d63c4e14452db8905ee4e1df9fe5c43161311265744c2bbb68b51b8f2e324959350b26dde799a7c5353d3d67d15edbb14d17f729cf7209870a13c2deb650332592d04a0b3c30772ecedd93f045f688d1f4",
        "eth1_data": {
          "deposit_root": "0xb53d08908c6c7946b23c37b826848e624a1484d18b5e02dffb738137cad63d29",
          "deposit_count": "37025",
          "block_hash": "0x5adeb26c3cbdd173235c4939bbc1ab4f693787ec11b2651dea22e472e13b02c2"
        },
        "graffiti": "0x6c69676874686f7573652d676574682d30303100000000000000000000000000",
        "proposer_slashings": [],
        "attester_slashings": [],
        "attestations": [
          {
            "aggregation_bits": "0xd7fffffefbdbff7f7fcdfef7f7ffdbefdfeffbd5fff6dde7fbff7fbffef7ff3f7ff5bfefdf7fffefbffffdfbfffeff767ef72ebffbffff2ffeffffb7fe77ffeffef7bddfe5d5ff93e2fffffffeffff6fffffefdfbeeefefaffeff7fffd9adbbfff76feffefdfff7bf9ffffffff7ffe7bffaef7efffb7ff7f65efffdffef7fffffffcfdfffffd7ff7f7ffffeefdfdfeefffdd6fd9fa7ff7fffebffe4bfbf5f7ffffeff7bead73feffefffef8dedfdfe5efdfffffefb5feedff5d9b3ffb9d7ffdfff7afffddfbfffeeafbeb7fdf7ffeefcfeffdfffbfffdfed7ffbd7ffe3cffeefd79cfffc7fe7bfcf7fffffff2713fdffbfd7f6bbfbbdb7bfef7feffff7ffffdf9cfefff7feedf5ffdd7fdfeeafdf7bee6f3bbff2efbf9dfe5fffbffdfec7ff7ff7ffffbffffffdfffdf7f777fffff9ff7fffe7fffff5dffffffffbffffff7ffffbffffffefff7f7dfadff7eddffffffeefffdfdfdbfffe7effbfefeedfffff7ffeff7fffffffffffaffffdfffeeff7fff3cfffffdff7ffbcfffffffd7ffbfffbffeffcbfffbf7fb6dfbfffbffbfffdfbf7bebffdffffa7fb7befffffbfbffbdfffefffffefdfbeffffffef7fefdbfbffafffedffff7fffdffe7fefffdf97ffffbff3f75d7fdfdfffffffefffffafffdffffffbffffdffffff7fffffffbfffedfdffffffbffbbfefbffeff57fffffffffbefb7fff2f7fff7fff7fbdbffff5f36ee1b5fffff7f5fffffefffff7ffffdffffdfbff5fdfffffffffbffeff7fbfbfffbafbffefffdfdffffbfe3f7ffffff7fbffffdffbadfdfffffdfbfffdbdffffd7f7ff76a7bfb7a7ffffbcfdffdfffffffdfbf4ffbfbeffefff7bffffe7f3ffbffff7fff7d6f9ffffdfdbfffbaffbeffeefffdfffeff7ffffffffe2fffbbfdfffdff7ff7fffdfffeffdfffffdebdff7ffdffffdfef7f9d3f7ffffffeefffffbffffbfbf7fefb7ffffe7ffffdbdfe7fe8e6fd7fdf7df7ffffbb7ffcfbfddfbedfbffe7dffffffefff7ff6ebbfeb4ffbfffbffffff77dffbf7fffefff7dfffeffb5bdefffbf6bfcf6ef57fffeedefffff7dffffffdefbbfff997f37ff7afaff7ffffffeffeff9ddfffacaffcfb5ee77afdf7ffeffffcabfb7f9ffbf3ffdbdf6fe3ebbefefff7fe7ffebcffeeffb3fffefbff8affafffffdbd3ffbdb7bfffdffbfbd8fbabffcf67ffffefd1f2ffd7dfff1fffaf7f7bfb7d9efeebff77fffbdbfbfcafafdffeffb7feefefff6f5f7f7fffeffffffffdfffef7ffffeef37fbff7ef37ffff3fffbfff9fffffffd7ffffff7ffffdffeffffafffffffeeffff5ff5ff6bf7fff7bdf7d2ffffebbdffcffffefff9ffeff7acbfffefeefcef5fdfffdfe3ff2bdfbedbff5fd3eddffefefeffef4ff7effffbff7e7ef77b7fdbef7d77f7ffcf7fffcfefeffff2dfbbffebdfb7ffe5ff3fffffdf37ffe7ffeffffffffbffefffffffffffffedfffddfffff6bdbbdefdffffbbfffffe5fffbfeefaefff5bdffffffbfd3ff7b7f7fff7febffefff5ffffffff9fffffffff9fedfeffeffffffdfffffffffffffffffffffe7feddffff7fffebdeffdffffeeffef7efdfffffffffbfefefeefffef7ffffffffbf7f7ffbf7ffa5ff7ffffdffffffbfffdf7fffffffff9ef77a7ffb7bdefffdff7fff7bfffdfffffdbfbffffffdffffbff9ffeffffeebbf7fffdffbbfffffdfffffd7bffffd7ffbfffefff7ffedfff3fbfffff1dfefbaefff75ffdfffff7fefffbfffffefdef7fefdfdfffd9afaefafffffbffb6bfdbebaeffdfbefbf7fffffff76dfddffbefeffffffffdfeedfbfbfdffffff9effbfffff5bfedffdefff7bf7ffebffdf7fffffeebfefb77f7ffefefef6ffffdfdfbbffffff7ffbffffdf6f96f33fff7df7f75ef9ff7ffffffdffffff7ddefffffff7fdfefffff7fff9fffdeff77fb37bfbf7f7fdfedffddfefeffdff7dbffffffff5f6ffbfffcfbf9fffffaeffebffeffff7fbfbfbb7fffebfffffffbfcfffbfffffffedffddffffbffdf7fcf7ffb9ffffff7ff7efdffbffffddeffffbffffffdfbfffffffffefdffff6bffebffffffffffffff7fffffefd777ffffefffffffffffdffffefd7fbedfffffffdfffbf3feffffbebdd7ffffbeff7fffaeffffffffffffdfffffffffffff7ffbff7ffffdeff9fffbfffff9feeebffb3ffff7bdfdc257feeeefef77ffd6ffbdfbfe5bdbfafeef7ffdfabcfbf7b7aed6fffbffff7ff9fb7ded7ff7ceff7f77ff1f5eff7fb6fbfffffaf2bf9b77fdf5fff6fcfffffffffe7db9afffffffbfeff7ff77be7d5bbfffffdff7feefffffeffffd5ffefffeffdbbf7fefdfdddffff7fffbfffdffdfffffef6fd7fdf7fffffffbfff7f7ef7fefbfebfefbff7fffdfdfefbbedbff7fdfffd2f7fffffdffffeebdcfff7ffffdfdeffffef7ffefebef9fef7fffeffdeffbffcfafbefbfffffff77ffffdff6effbdff9ddebe7ffbffcdff7f9fffeffffff7ffbffffffb9fffefffff7feffffeedfeffffffe7fffefdfffbffdfffffffffdfdfdf9bfeffffffefff7ebfffefffffffffff7cfffffeffffffbfff8dffffcdfffffdfffeff77febdeffffffff7ffbff55db1fffb3f7fff7ef4fcabff7ff6efefd6fd3faf6ffd7fff7ffdf7ffd97fffefefffff7fbbfefffffa583fff7dfbf7df7d7f63fffecfbadfeffdfdffdbe7daaf5ab7eedc9fd1effefdfb7c1fbdb3edffcff7fbddbdf9eefd6df5f2e59ef579daeffac7ffbe97f7ff2defbedf7feffffb7eff7de7ffdfdfedb7ffeffadffbfffe9dfff3e3fdfffffdfeeeffffbfffffffffffffffeffbfd9fbaffffffffdbfffdefffffffeffbf7bdefeffffeffffffefffdfdfbf7eff9f7fffdb8efffffffefff3cf7fedffffffff7ff7ffff3dff7f05fffcbeb7fdf2d7bd7cd4fffff3fd67cdfbf1fffbadbbf73bdfd77ff8b7fffffbeecfeffcffffffbbeff3bba6f7fffbadf7ddf77ef79d9ff5fbfeeceeffff95ed7e7fff4ffbfffffffffbeffebffff77fffef7fffffdfbdfb7e7bffffdffeffeefebfeffefdbffffe6fffff6efffffeffdffff7fef7ff7fffff7dffffb7ffdff87f7ff9bfbbdef27dfffffffffffffbda6fdff5bf37fff7d3ced1fffafeffffd6eeff76f3d77e5fffb7ffb7eff77b7ba7ffffdfbaf7bffd77ff39ffcfedffee577accfff3ffcf7e3f77ffff7bdff7fdefdfffdfffffd9cf5fe4ffffdaf5dbdffbf7ddb6f7fedfffffeed7ffeefcfecdfd9fffeefb9ff1fef7dd7affffbedf6fdfbffdfbff9dcb7fedfffdfbffaff9fefddfbe7ecfdbffefbfffefffddf6ffdfdffdfdff7dffef7fffffbfdffaf7dffffbff57ffff13de7ffffb7f7ffffdedfff6fff69e3fb7effefbbf5ffdfef7fffcfffdfffeefbbefefffd3e7fbd3fdd77fffffffbfbfeb7b7ffffa7ddf5efefe3f7fdd3977fffe95ff3bddfbfbfffffddfff3afffff36fb6f7ddf7efefdfbbffff3fffafcefffbbf7f8efe7fdffddfffe3fbde7bebdf55ffff7f7fbbffcef7fffffdfbfeffefff7ff27f25bef7fafb5c7f33f79ff7fdefbfffcf7ffbfffffffffe73d7bf8dbfefffdffef4fffdf7fff7df3ddfd93972eb7fffff7ff5effdfffffbfbb7baefff1d7befb5ffdfffff77fffffdedffffcfefbeffdf7ffefffbfffed7bbfffffef7adfffff3effffffff5ffffff77fff7fcfeff7ff7fffffb7effdff7ffff6eefeb7feff7dfbfff7f7ffdfeffef77fbf47dffffffffffffdffedfdffaffbffd5fffbfdf1ffdedfff7bfff1f3fddbef7efffeffeefffaed3cf7c4d7c7ffbfeffdeecefffbfefffbdfff9ed53a5fff5f7dfdffedc7fdff7ffedf7efeffd75dffeff7fffbbdde57ffe7fc7f7dfeff7fa53ffebbefefefff7fe6feff3fffffebecdffe8fffb8f74b75faffffff9ffffffffde5bb6ffebbffbeffefedbcbcbdfff7fffffb7fffffffff3df9fffcebfbffdffffdfbedfbfefdffffffffff7ffffd7f2dfbfffe6ffff7fafdfffbffffffbcfdefbefddbffbfffdff5fffefff7fe3fffdfeeffffbcffefffa7ffeafffffffefacf7ffdfffffe3bfeffef7f7ffefdef9ef67fbdbff6ef9fff3fecfbfbfbfffff7ff9f7baeffbdfff3fffd9dd6fffbffffbfff5ffbd6f7fbfdebdebfffcf6bfdfafffff9f7bf7fcfffffbdfe77ffebfde7afdffffeefffebdaffffffdf5773ff6f717fcffffbe7fd79ffd7efffffbfff7f7ebfffff6d7fdfdfc7f57f77ffefeffffbf72b79fa8ff7ffbe7cdec8ffbff6dd6feffed5fd7aa749e65e17fd9df6bb7c809b9930ef5d79fcb3f3196afbd734adffe369dbddbff957f3f6f6d7dfeedfd63e75cbdf636fd0bf87d1ff1dfedccabd7dfffffdfffdf6ffebeffffffffeffffffbfafefffffffdfffae7bfff7febfd7fbfdff7ffbf7dfff7ffbbbf6fffaffff7fff7ffffbffffffffffe7bf7efeefffffb5ffbfff77fbfffdfff1fffff7fffffdffbbdeefffdefbfbfffefffffbffeff7fd7d9fe3ee75ffffffdff7fff7f77efffffebffdefcddbbfdffdedffe7bfbfffbef6fbf7fcf79deaffffffffffffffff7fff75fdf2ff7fddbf69fbbdfefd9ef6ef77fefffe1fbcef7fffef9fddedaaefdf4cbfeaf6ff765fdd57dff45efbdae7dea47affcaf77e7ea3fb7f7d7f77dfdfd7ffdd6bfffff5bfbf77ffdfb9ffbf7f6ffc66fb7d7bbfbfff5ff0fff7fbdfefbffd7bff7ffffafbccfffdffeddf77ffeb7fb7fffe77f7f6cf7fbffbfb7ffdcbf3ffd74f773cefffffdbbffffdbfab3db7effeffbeffbffbfdff7ffdffffffffff7fdf7ff9bffffff7dbdeefdfffefffbdcffefffffedeffbffffffbfffbe7fffeefff77ffeffffe27feffcfffffffff9d7dfaffffffffff5f7ddfffffdffff5dfffbfe7fff7fffb7cedbbfbfef7f68fe6fff7f7ffe5f76dffffbbf7f7f7fef7ff77dfbfbfef7ffa7fdfffbfbfbf7bffeffeffffffffffef7edfff6ffbdeefffeffffbfffd5baffcbeefffffffdfffffbf3ff7fbefffff8ffefbffdf7bbbdbffffeef7fdfbeffff7efebef7dfdefffffd9fdffdfdf7ffe6fbbfdff7fffbfdfffffefedfefdf76dffcfdbeffa9fffbefffeeff5fdf73dbfeff56efffbfff7fd7fffe8ff76f7b3dfffff7fff7fdbffbdfb7f3ffffa3dffff7fdfdfffdbbb3fffbfbffefb5ffffffdef7afc79ffb3adfbdffffdfb2bfffe7f76bbfbfffff57f4dfb7f4dffbdecf7e7ffd65db7dbfd7feaffb9f6caff7d3ff7ff7bfabffbfffdc3ffbff9bff77af1e7fbf9ffedbf35fbff73f77ffbbfedfe6fedb3fd7effffd3f7bffff7ef7fffeff6ffdfffbcffffffdbdffeb7fddf7f5fffdffffffffffffdfffff7fbfff9f9eab7feeaffff7fb7efeff6ffdeffefffff6fffff75fdf6ebffefffffedfebdffeffd7f7bfffafffffbfffffff7f7fffffffffffff4ffeffdf7ffbb7ffeffeff7ffdffffffffeeff7ddfff73dfffeffffdffdfefffffffdffffbf7fff7febffffff7fdfff5bfdfdffdf7bffffeffffdfffbfdff7dbfefefbbfff4edfdffdf7defdfffffffefffffff5bbfbf7fbffff5ffdfbffb7fff6baeffff7fdffbefebffffffa7dfffe5fffeffff5ebff5fffffffbff7bb8effffff3df9febde7fdf9c7fed7ffdefdfbffc7ee6ff77755fefbbebefbef77a77dff9fffe9d9e9f8f4feab7f5dd7fffdedeff3f3d97dfdf5f7ffe5ef7fffbff15fffff7fb73ffbfbeffffeee7ffef7ffafd3bfbfbfffeb777ff5ffe5b59fdbff6fdf9f7b7efdfff5fffeeff7eff95f7cefdfffff77fefe77ffffdffdf7fedfff7cf2ffdff5f7bfbdffffefffd7dffdfffff7bf3fffd7ffff76f7bffefee7fedbffffffcffffffffefff7fffffdffff3fffdffffbcfffeffdbffff7d7fdfdfffefedfefffffdff7fffffff7fffbbffffffffffffffb77fe75bf5f5b6dfbfbbf7ef7ff7f7ffef7dfffaffdb9bf6b9affdf5ffffe5fbdfeffffefffbd7ff7ffdf77dbde7ffffed7ffff79fd7fbee5fac27febcffffbfe37a375f8effeefffeffffffff7fbfdedfbf5ffff4e37bbefdfffdd9bdfafee9f7f77fa7dffe7feeffd9fdffbbefdffeff5effeffefdf7fffe7ffffccfbfffffffffef6bdff7ffdfbeefdfdfbfffff73febedffbfedff7eff9fbffedffefffddfddfd7f97f7ffffffebf79ffdffff7d9cfdff7fdeff5f7f7ff5dfdbfffffdbbafafffbfedefddfef7dffdff5df3feeb7beff1bfffdffba76dfff7ff7fbfeffbfbdffbf7effffd3efff7ffe7fffdddffdf7f3fbfbbefbfffffd7f7a7ff7fbff6febfbffbfbfdfffefffffdfeaffffffff7fefffffffbffffffffb7dbfadff7dfd6e5fff3fabdede9fdbfff56fffbf5fff5fecff1fdfef3efa79f7effffe79f3fdff73fb7f9ffbfebfbf8f3fff3eff97febfdfcdb76bf7fef7beffa27debfdfe4ffdffffbffffff6dbffcf7f1ef9713ef3587daffc7d2b5e7ab7fdfb3ff8dffebf6ffc1dfff5e118fe7f56fdb7bd6befff5ff95bd7ee97b73ea7d59fbdbe272ab7fd79ffaffaf67e3fffeffffec7edf7fffffefebffffdffffffffffffbffffffcfbfef7fffdffb7f7fff7ffffddfbbbefffff2bffefffff7fffbfffffff7f7fffffefbfebffffff7ffeffffffdfdb7efffdffeefffeffddfdffdf1f",
            "data": {
              "slot": "944792",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0xa47eea8758d02f1922e1c5c28fcc19a5da3579c407f67a8a69b3abc40186bdd5cfb02f80d85c17b0b5cf832d8f12d5a70824e5a1b70cb2b555cdd8c02ee3c1f964744f40dcc43b4c8c39864c52cc9c0ef3e827b6c73cacb8fa4a0369d53e6f88",
            "committee_bits": "0xffffffffffffffff"
          },
          {
            "aggregation_bits": "0xfdfeffeffbbfdfff7f6dfbfdfffffff9fdffffffffefffffbfdff7ffffffd7fffffffffffffffbffc97ffdf5ff77ffffffe7fdffbff63fffffeffbfddfeffffeffffffedfffbbdffdffff7ff7fd97f87febfff7f7ff75fcfffdffffbfefffffdffffdffbebefdfeddc6fde9f7effbfefb7fef7fffdffbfefffeffffcdeef97ff76effffffb77dd5dffff9bfbfffafbd7fef7fdffbf9ffdfbff7f7cfff9ffff5fff7eb6bfefdfef7dfdbdfffdfff7db7fcdfe77feefff7dffff7fff7f3ffff7ffffb7ffbfbdffdeefffef7f7fddaffd9fadabfd7bfffdfdeef5fffffeffbffbf33f7fbffffff7fdfffddfaffeffebfb7feff9ffb5ffdbfa7bffabefdb8fdf1fbe77efbbffbfefeaeffffbdffd7f7ffbb2d7fbfffefffdfcfbfefddfbcedfd7fffeffffffffeffcffd7dffb6f7fceffffcdffbffff77fbfcf7fff7fbbf7ffbf7fffbfffffefeffff7fff4f7ffeffffffbffbffffffef5eedffeff7fe1efff9d57fefaffaf3efffafff6dffffbdffdffeffdfdeff5fb7f75ffff7ff7efff73ff7ffbfffdfff7bfbebf73bef6fe7ffffebfedffffffffdbbfdfeffcf6fafdfddfffd3bfffbfdfaff6defffedfffd4dfdb9fffdff777eddbffbda7fffdfffffd6ebffffaf9fdfffffdfdfff77bfffff73ffffffffffffbf77ffff76effdfd777fbffffefbffff797fffffefffbf7ffffeffffd7f3fafbfb77f9bfffff7ffffbfffffffffbfbfffffdfbfffff5fffffebfefff7fdffbff77fffbfffdcfff7fffffdfedfef75fbbfdffffdffdfffff7fffdeefffefcffbdfe7fffffffeffb7dddfd7bbfffbbffefff1d7fef6ffffffffffffdffbfdfffffcf9fdffffff7fdffeffda7ffefffeffff9f7cfdfffdfdff7dffffffeedfffffffffffbbffefffefbeffffffbfffffefebe5ffff7effe797fb7f3ebf7fff67dfdfbffeefffdf7f6f7fffff7ffe3fffffddfdfddbfffefffbff9effd7cffffffff7ffff9fffebbfdfffffffffefbfbf5ff96fd9ff7f7bffffffffeffdffbaffdfffedf76fffaff6bfbfdfebfffcfbfffbffdeff63fff3effff3bffdff6ffef7ffffebbeadff7ffffffffcfca7bffffef7f6ffeeffe7ffdfd77bddffffd7fdffffbecdfe7fe9bfffffebbffffff7fffefffeadffbfffbedffffffbffffffff3dbffdbfff777fbffffffeffff9ffff9ffffff7ef7fffffffffbabfb7ffffbbbbf7ffefff7ffffefffefdff7bffffffffffff1fffffbeffffbffbffffbffffffffdf9fddebee3ffffffeb6fdff7ff7ffffffffffff1fefffdffff7fdfdffefffdfbfffeb77d7b7ffffffffbfffefffefffffffebfbfffffdff5feef3bfe77d8fff7fefffed7dfbfeffff7ffdfffffd3bfff3fffffffffffffffdfbfffffffdfffffb9f7fbffffff7fdbeeffff9ff7feffdefd7fefdfbbfffffffff37df9beffdf3fbfdfbbf5fdffeffbfffffffff7fffffffffeefffe7f36fffefef9fcffffa7cfffffbbffbedbbffffefdff7dfbffffffbffeffeb7ffc7fe7ea7f77b7ff7f9fff7dbffdfe7fddffffff5df757bfffdfbffdfefffffffdeffafff3f7feff5f7bfb5ddf7ffbfdf7ff7efeff7efe6f7ffbafef7fffdbeb6f7ffc5fdffbdffbeffbffeb6a9fffbfffbfffefefcffaefffffffffefbff79fdfbfdfdffef9bfa7fffbfdff56fdeffffffaff7dfbbed7ffffcffeff77ddf75fffddfffbeffbcffff7ffdff7ffbefffa7ffeffdfbfd7bffffff79cf7ffbfeffffdfefff6fffbffff7de7fb7ff7ddffffff8fbf9edbedffeefffffddfffffefdffeffaffffdeeffffb7ffffefeff7ffd37ff7dffffeefdbfbffeffbfdffbff2fffbff7fcfeffffff3ffffffbffbf7f7ff7fdff7dffff75ff7fbfbfffffe95f7fbfffffe9fffbf7ab7dfdbdfffff7efbffbff6fefff9d7fffffbfffffffe7ffffffdfffffdffffbb7fff7ffdf1dfffbfffffeffe7cfffffdbffbfffbff7ffff7ef7ffbf6fafbfbfffbfffff7ff7fffffffbffbcffdfffdbfefdfbfffe7eefefffffbf7fd7ffd77fd5fdffe7fff7eff6ffffbffffeff5fefffcffffffbffbffff6f7ffbfdfffff7b9bfebfb7ffffffdffb6bfffff7ffefe77deffefdefffefdfb6ffd7ef77fffffefff73ffbfffffffbfff7ffbfefdff7d5fffffefff7effeffffffffbfff37efffff7fe7dfffeff7fffffbf7dfbe6fbfedff7fff7f7ffbffdf3ffffdfdbcfdbd7f7ff7ddfbd3fffeff97fffba7fe7fdbffbf77f3bbfeefffffdffffff7fafbffbf7ff7d7eafeffffdcef7fbfffffff7fff77bef7f7b3fbf77fffffffffffbefcfffff6ffffffeffbffffbffffffffffffbeffffffffdfefffffffff7fbfbffffffbffffbfeaedfffdebbfffffdf7fffffb9ffd7ffff7feff7fffdff3abf6efefbbedff5fbefbf7bdf37ffff9fefffadfffb7fbfee77ffbab5b979dfff7c6fffebbff6ffbbabfffefbffbdffffffff7ffeffafeffffeffefeff7ffee3fffefffefff3bffb7edffffbfcfffdffef7ffffffffff7ff7fffffefbbfffb6efdafffdfffff7bfdefbffdffffffdffefb7fffffffdfdfffdeffffffffffffffffbffcfffdfffffbf9fff53dfffffdf5fffffffffffffd6f77fdbff7ffffdffefdf7fbde7fbfeffeeffffff7effaff9fd7bffeffefdffbeef7dffffdffeffdcebfff57bfffeffffffffd7fbfeffdffbeffffffffff9fff7ffffaf3fffffdb7fffffeffbfbadffffffbffebfffffeffcffdffffffff7fffff7ef37ffefb73f3fffde7fffefbfffb7ffdbfefffaebbfefff5fffbffef7ebffffffefffffffffffffeff7fffe7ef7bfffbf5f7afff7fffffd7dfff7fffccff3fffffeffffffe6fffffffebffefdf3fffffffffbfffbfbbfefef7fbf6ffff3f3f7ffffffbedbfff7fff37fdffffdfbf7ffd67fecdfffbf6ffbffffff7fff9feeff9effbfdfdfbbf7efffbb7efebffe5bfffdddfdfffffffeffafdebefbff7ffd9f9f7bfc3fbfdeedffafffbfbffdfdf7fffeffdf7fdffdf5edbffffeefdbddd7ffffaf7fedefff7f7dffff77f76dffffffaffffffebfffbff777e7df7fcfeffdffffffefffff7fe550fff7ff6fbffe7fdffd7effffdfbdf7bfcfaf7fb7ffffff5fbf7ffef3fdfbff3ea9fefff7ffeffff17ffffffbb7ffdffebfffff9ff7f9ffffffd6dfffffbfffffdfffff9fffffffff9ffffdf7ffffbbffffffefffefdffff7fffbbefdffffffdfffffffeffffbfdff9fffeffe7fffffafdefedfbffdffefeffefffff7ff4ffff5ebdf7f7fff7bd7f7fefbefbafff7fffbfffafffedbaffe9e77ffeffff6fbfffdbbafee7ffdffff5eff2fbfdacfffddedff3b7fb77fdf9ebbfaeaffbffdeffefbbffffbfbfffdef7ffdfbfbf4ffbbfbfffffefeffbd7f8ffb7fffeffffffffffbfefdeffffff377dffdcefff7fbffff37ff7afffedbbfbfffeff7f93fffbfffffbbdfff4fffefffbffff7bffffef7effff3fffd7efffeaf9df7f7ffdf7dffbfdfeffffff7f77beffffcff7fffffbfffffb7ffbbfffefffffffffffdffdfeffffd7feffdf7faefffffbfffbf5fff9fff7f6ffbffdffffffffaffafbffdffbbebfabfffb75ffecffff79ffffffffeffe7ffffefef7ffffdfffeeefdffbfff5fdb2f7fffbffffed9fffffbffef3bedfffffbfffee7fdfffffffff6b7bfffdffffffdfeffffffffdffffa7efeee3bdfbdfffffef7afefffbfffbbbbfff3fefbfefeff7ffffffbfffffcfffffffff4ffefffdf56ffbfffdfb7dffffffffebf7ffeffffbfbffbeffffbfffbfe7aeddeeffffefffd76bacdffdeffff6ffffbb6fedefffff9fffffffdfffeef7fffcfeffffeffbfffff7fffbffff5fdfff7f7ffffdff7ffdffffff7ff7df7f5dfbffdff5f7f6e7fffffdf7fefffffeeff6ff9ffde7d3bffbe77dfefffffff9fdfffbffff7fdf77ffdfff5ffeffe77df7dfffefcffffdff7ffffff7fffffdbedf7ff6ffffeffafffff1defeffebfeffffef7efeffede5fbff7ffe7fecf6dfffbbffbf796f5ff867faeffef17bfbdf7fff7f7effbeeffffedffff7fdf75ffadbfdfeffffffeffffdfefdfffffffbfffffffdefbfff7ddffffffffff5ffffff7ffffff7ffffffffbdf7fffffff7feff9ffbef77fffdffffdffdfffbff7fffdf7fbfbefe7ffebffddfffffefbfd7ef9ffbfffb3d75efbf7fffffdff6fbdfdfbffefe7e7fffffe7fff7ffdff9fdf3fff7fffb6ffffdffbecff7fbebfeddfbfdffb2fdffffeffefffff6fefbffffffffffffdfdbffcefff7fbfbf7f37fe7ffff7ddfbfffefff5efffffffffddff5777feffde7ff8dfdffffffefbffffffe7fffebefffffefffffffefd7fefffffbfffffffffffbfffffdbff7f7f7ffff7ffefeffffaffffffebfeffffffff77ffd7fffff9efdf7cf9ffffff7fef6ffbfffffdbfddeff2fb6faf7bbfffffeffffff3e7fdfffb3fdff97e3dfdffbffffffffdf6ecffdfdfebf7effcffffffcee7ffdff7ffdf77c83fdaffdfff7f7fffefff7fbfbffe77febf76fedefdff5fbd7ff3ff7fbfffffffdfeffffebffcbf7ebeffdbfbbfbfdfffffc7fcffffd7fffeffb7ffff67ffffefefefffedffefff3fffefffaffbffddfff7fffbeeff6ff7fffcffffff9dfbf7febf7fffffffeff2f7bfff3ff7fafffffffbbfff5fffeffdedffbffffffefdfefffffbfef9dffeffbffbffffffffdffabfff7effffbbf57fffdf6fffffffff7ffb7fffffcfffbcfefffffe3fbfbffffefff7ff39ebfbefbf7fbf7fffabbfffefff7f9bf7ebf1fdedf3ff3ffefa77fbbbdfbeec7d27fdf7df32bbf7ffdb7fff9ef3ffedffff7dfbfffffdfff7567bdb7bffbb75ffdffcfe3defe3daff977bfbdfffff3efffe7dffefbffffffff5fdfff7ffbefebffe9ef7dfffffddfff7ffffcfffffffffbfef7fffff77ff9ff37fffffebbdffeffffffffebfafffdfeff97f7cffffff7ffb7f7ffdf53fffffdffb6effffedfffefeffbffffffbfff5bfffffffdffffffffff7fcffff7fffeffffedfffedfffb7fffffffff7fbfffbedffffeeffffffbfffdbefc7ffffff7ffefbffffffffbffcfffbdfffd3eff3ef7efffff7fdffdfd7ef7bf6b7fffdefd5fffff7fd7fefbfbbe7fffefff77fff59fffffddffffd9fdffffbfdfff7ffffff7ffbbfff7ffffedffffff776ffafdffdddfeff5ffdebba57ffaffffffbfffefefdddbf7ffff7bfffeb7bffe75fdefdff9e3fbbffffffffdf3ffd7ffff7ffbfffef7dffa7febbf5ffebb35bf3fdcfffebfffffeabba7dafdfff7febfdf76fffbbffefecffebffffbffdeffbbdffffadeffffef9ffefdf5dfff59fffbfbbfffdd7fdffffef7a7ffff56d7e7b3ff57ffe67fffdf1fdfe7ffeff67fefbfffeef77dbffdffbebffdfffdfeffffffbe1ff7fbffdddfff7ffbff30ffffffbfffdfffff7feeef757fb9ffbfd7ffffdfffeffffffffffefffbffffdbffb7feffffffddffdff7ebffffeffffe3def7fbfff7b7edffbfff7dbfdffe7db7fffffffbffefbfddb7fbdfffbffffffff7ffbfffffbff7fbffeffdfbff7fbebfe736fffeff7bff6fd6bf7fffdbffffffeffdffcfbfb9ffbfcbefedfaffffffffbefdfffb3fffffb7f7fffffffff5fdffbdfff6ffdbbffbcff6bffeffeffffdffff7efddf3efeffbb7bfdffdfdb7ffbfeffffb3ffffffff6fbfffef7fffffbffed5eddf7df7f7fefefbfefeffcffbffe7df9fffbdfffffffffefaffbcbfddfeb7f177ffdff7ff7f7fef7fbffbfdfe7ffbfffffffffffdfffbbedff77fffd77fffffff7ffffffbffff777dfc7fffff7fffdfffffe7fdffdcfedfaeefffffdbfbfeebc79fdaaf7b7ee777effffffb77fefffff67f5fb2f9bffffddcf6c7dfbfdfffffdffffbbf7bffffbff9efcfffffeffffffff7fdffeffefffffdb2fbefbffbffffefebff7ded75bfdfbffbfb5ffff7ffdfff7feffffffbff7dffffffffffddddfddbf7fffdefe7fffffffffffefeffffffff3bffb9ffffffffb7f7eefffffbfffbffff0edffedd9efffbbf3ff7fb6ee3fafdffffeffe7fe7feffffffbcfffffffffec7ffefefff7ff7f9eff7ebf7ffffedbf7fffffefffbeffeedffccff6fffeffdcbfefcf74fdff7ffffb9fffdff94fbffd7ffff7f2f7fffffffdffeffbfffffbfdfffebef7fffdfee3ffefffdfebffffbfffffff9effaffdd7f7fffdfffef7fdfffddfff7ffefecfbffbbbfffdbfdfff7ebb7feffeedfffdeebcfbefffffdfaffff7eeff7fffffffffbebfbfdffffffd5f97f7fffffeffcfff9ffc7ffbdbbdfdddfffcffadffad7fbef7fffffbffffdbffff7f67dffffef7fbf7fbfffc7edfe7def7fff7ff7ffff7effff7fffffffbe67ee97cffffbfffe66ffffedfff6f7ff5fffedfbfbccdaffffa6fb9efbffffb9fbedffe7feffbfe3bbfffdffffd6ff6ddd7ef968ffefabdffbfff9ffb99effcb7ebff0f77e57d36bddfefdfff7f3fbdf2df74dfc77dfdff6bf3f37f9fc77ee7adce357f6d69ffe5dff5de95fe77ae7afbebfbbdffcffff9ddfff7bcdfffffdffffddfffffffffffeffff6f9fcffdf37ff7ffffffffffff77ffffffffffbffffff7fff7fffffadfeeffee77ffeff3fffbfdb9f7ff7ffffef7f7fd7feefffbfbd1f",
            "data": {
              "slot": "944791",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0xafd0d5b26606665a398b2ec8efe47875e69157d2dfe53faf141904888f8fbac0f917ea9a7e73a05b453317e367323cf515e186d38043b9765e5b919321a745549030bad702354012798e64024ba7709a77b22416c5687437c7a05ab4559ed8c1",
            "committee_bits": "0xffffffffffffffff"
          },
          {
            "aggregation_bits": "0x0051050801195e00210404a840030104000007400280400284001034c60100014490000821000001484000408082a110c00480008b11040000000a100080010000023000100000000a",
            "data": {
              "slot": "944792",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0x8fa549c6c26e38c2d98c2b2099aa2da535314442c9bb29af5b659abbf478454814eb1ee0bfde258be1231a98da2f0bfd13d07d5e8312b783eece6cffa8bce91c87b6bae63e9204b7e67f74d3d262a5bbed2fc25d754b964ac6adb0f899650871",
            "committee_bits": "0x0000000000000040"
          },
          {
            "aggregation_bits": "0x010405020000010024090400044012201001000280800000000040c02408004006802a02001240051000144208480840000a0022000201980800050110010050000428010000000004",
            "data": {
              "slot": "944792",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0x8404fad15d6e3fb77bbd081a3e9e9ddd47f29631ca6239e92e94bf781c27ae575db53c48c4742132cfcdcfeb826d99030c4eaa19d314160ddb8bfec62134786d29f3dd78992dbfb51ac198cfc606a5c385b53139b6c313788214e37e45587a18",
            "committee_bits": "0x0000000000100000"
          },
          {
            "aggregation_bits": "0x041020c280000804000000c200c40016100000600125041000402000000080811a40004020040040081908008150001080421401210500104014021402108100080008940000000808",
            "data": {
              "slot": "944791",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0xb8837656a1d52e1f2b39a12f241fff4b0935af87184cdb9b84f3ad497b7ca2c3b52463eef58f14278bb5fec956829abd19d6e3d14278241d50a8bc56ab774cd8078147da66b8c11a4fbf2fde090239d43be673cdc31f58a8a6c3b40c894a2bfa",
            "committee_bits": "0x0000000000000040"
          },
          {
            "aggregation_bits": "0x12cf12f7e7420af3bfa944effd8d34c77b6d86775aef70a06866cafc6545505e6563eacd4fd655e6aea66cb7bee2fb6fd7d13f4ff357597f9e0d3f2ff9c5bb9e4edf5fea733b837e0c",
            "data": {
              "slot": "944792",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0x83b0f685bf6735f5b2a6203247893f872c4d8bf4d1552f74eb111d8e37579aced56be1e88990a44477d125a7bcdffb3b130e8637d2afe83440accf510da6031c25c8842dfbe26d158355f72175df4fe71077d6840bae02f9f494aef917fb8052",
            "committee_bits": "0x0000000000020000"
          },
          {
            "aggregation_bits": "0xf2723398199e3f0c15d27532901023bc15202040006252314648c60032c4e714f546a2401141921805ab18d145981400c48b7a4043ab049d00286c2331405f0849e0081e8204c3e809",
            "data": {
              "slot": "944792",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0xb7a959b3d1c25de9526d6baf55b805b859a97e6754a37a0ae03ae57bccc52b86eb4edb1c22761279fe4cf12111c1b38409a82273d33cb0c97b09f331289cdc8fabf173937337360b4916cc4ce92b1e05d5617a82ba294797912de97a984be80c",
            "committee_bits": "0x0000000000020000"
          },
          {
            "aggregation_bits": "0xb599216f6b47161a06149442cc04e3db914217c8d059b08a0e233dcafb851ad65bb2d659b4290ce6a0f930162a09f3b11a75c01129f9a50e05352e8eda048784fe257512edd917010f",
            "data": {
              "slot": "944792",
              "index": "0",
              "beacon_block_root": "0xea39d11b258bf2d8faa6a5da44dd5dd42746453933a31404df330e3fef511084",
              "source": {
                "epoch": "29523",
                "root": "0xaa7312b9b5e6feb182d1872df738ff35aeb5382e0165cc02356d9f734ff9f78a"
              },
              "target": {
                "epoch": "29524",
                "root": "0x6e0bbe263cd33a861580166a50486526c7df613f03f08c73f238e54ec2147ef5"
              }
            },
            "signature": "0xac646c3db4d69c6526a1228bfe5c280c2818adb97621a176a7b17b13340f5843e3610dcb217a2aa8c5420999e9635b901214258b78c419e93a2624f8fa8315aad948fdfffeb7e3d570d6e50dfd2688a35dc642601a5b460565f54abd13ed7253",
            "committee_bits": "0x0000000000000200"
          }
        ],
        "deposits": [],
        "voluntary_exits": [],
        "sync_aggregate": {
          "sync_committee_bits": "0xfffbfedffffffdfffdff7fffffffffffffffffffffbdf5e9efffffffffff7ffff7fff7efffb7dfffffffdffbfbefffff9fffdffffb7feb77ffdfdffbffffbfff",
          "sync_committee_signature": "0x8435f4745c27abaf5d21937c7159b1d1f3691742a7615cb7493d45fc9015405849d7ab75f0e67800c0b51540cd57f0311021cbef4a10aa7f742c9a73cd55bf5a060a3c408d38f63f065b1b24c5806f20e234a9a92f690f86c808a63710d28cb1"
        },
        "execution_payload": {
          "parent_hash": "0xf598d016f0a45c82e56af5759042d4ca46739298a69156854e7f0cb18e0b4c02",
          "fee_recipient": "0xf97e180c050e5Ab072211Ad2C213Eb5AEE4DF134",
          "state_root": "0xc5e77088f9d64edc68ddaef4c646632cfbd2df3a3f5dd0b754e9579ba5b1b65e",
          "receipts_root": "0x0acbdd33d2a65cc22277a3ad7ea0276cf8e9a09c31a06930b1b779773b280c35",
          "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
          "prev_randao": "0x36e4302847a1e8617cae2592c2779ddebaa02f9405ea82eee1c318305fa89765",
          "block_number": "883631",
          "gas_limit": "59941408",
          "gas_used": "408923",
          "timestamp": "1753550916",
          "extra_data": "0x544f4f4c2076302e352e30",
          "base_fee_per_gas": "967310439",
          "block_hash": "0x14a7411377844142b3c9c8c3a47b390f422949b7453e206b8da066c2d7b790d2",
          "transactions": [
            "0x02f9011583088bb082093e843b9aca0084be891de283060c8194046fa9960e51cbb43d8e69ef315e4e76774bcda280b8a482ecf2f6000000000000000000000000000000000000000000000000000000000000000100bc33a71887d42309fcc9a2b14aabb225e9b2348d64570a91a5027e311934790000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000ab367c080a07cc08731cd31dfcf656a18f3cb82347fa534a647439d3ee875467c9667248c66a0205ba93d9ec5fa34792a3557266c090b940676b8a21e27c9ead65124e31984e0",
            "0x02f87183088bb08206b58506fd5c1d3d850770ac19f3825208947176811594dfaa403d1ab69625f3aecae7c664c58080c001a082e7785568c517745b97c5bb867dbb58a71eeaba3d992ee08fd0d69cb729ac28a02c10937b1acf04e475630d070f485ca3887f495f759e88dd7f8b6b69c85edbbd"
          ],
          "withdrawals": [
            {
              "index": "14040534",
              "validator_index": "1021188",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2935264"
            },
            {
              "index": "14040535",
              "validator_index": "1021189",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "49459291"
            },
            {
              "index": "14040536",
              "validator_index": "1021190",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2922709"
            },
            {
              "index": "14040537",
              "validator_index": "1021191",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2937818"
            },
            {
              "index": "14040538",
              "validator_index": "1021192",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2941147"
            },
            {
              "index": "14040539",
              "validator_index": "1021193",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2919467"
            },
            {
              "index": "14040540",
              "validator_index": "1021194",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2918814"
            },
            {
              "index": "14040541",
              "validator_index": "1021195",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2934111"
            },
            {
              "index": "14040542",
              "validator_index": "1021196",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2941638"
            },
            {
              "index": "14040543",
              "validator_index": "1021197",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2929570"
            },
            {
              "index": "14040544",
              "validator_index": "1021198",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2938140"
            },
            {
              "index": "14040545",
              "validator_index": "1021199",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2951878"
            },
            {
              "index": "14040546",
              "validator_index": "1021200",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2939451"
            },
            {
              "index": "14040547",
              "validator_index": "1021201",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2937940"
            },
            {
              "index": "14040548",
              "validator_index": "1021202",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2934910"
            },
            {
              "index": "14040549",
              "validator_index": "1021203",
              "address": "0xe5061fe5b4d0bd260f1ff80fa919e339f1f5c330",
              "amount": "2930079"
            }
          ],
          "blob_gas_used": "0",
          "excess_blob_gas": "0"
        },
        "bls_to_execution_changes": [],
        "blob_kzg_commitments": [],
        "execution_requests": {
          "deposits": [],
          "withdrawals": [],
          "consolidations": []
        }
      }
    },
    "signature": "0xa4ddc633d28af0140759e281541bfc56563ff4043c02bb22ef0815f12e7291136ceef9abc79a00915ac8b5a52c18640816ba4c149f0ba051c557c13c4ad3f55096c3999b1be4a670abe94731c7bd005e1d9d1d2ad2059ef0d7f75c5e860b39cc"
  },
  "kzg_proofs": [],
  "blobs": []
}`

	signedBlockContents := &electra2.SignedBlockContents{}
	err := signedBlockContents.UnmarshalJSON([]byte(signedBlock))
	if err != nil {
		t.Error("Failed to unmarshal signed block", "error", err)
	}

	executionPayload := signedBlockContents.SignedBlock.Message.Body.ExecutionPayload
	transactions := make([][]byte, 0, len(executionPayload.Transactions))
	for _, transaction := range executionPayload.Transactions {
		transactions = append(transactions, transaction)
	}
	b, err := engine.ExecutableDataToBlock(engine.ExecutableData{
		ParentHash:       common.Hash(executionPayload.ParentHash),
		FeeRecipient:     common.Address(executionPayload.FeeRecipient),
		StateRoot:        common.Hash(executionPayload.StateRoot),
		ReceiptsRoot:     common.Hash(executionPayload.ReceiptsRoot),
		LogsBloom:        executionPayload.LogsBloom[:],
		Random:           common.Hash(executionPayload.PrevRandao),
		Number:           executionPayload.BlockNumber,
		GasLimit:         executionPayload.GasLimit,
		GasUsed:          executionPayload.GasUsed,
		Timestamp:        executionPayload.Timestamp,
		ExtraData:        executionPayload.ExtraData,
		BaseFeePerGas:    executionPayload.BaseFeePerGas.ToBig(),
		BlockHash:        common.Hash(executionPayload.BlockHash),
		Transactions:     transactions,
		Withdrawals:      convertWithdrawalsToTypes(executionPayload.Withdrawals),
		BlobGasUsed:      &executionPayload.BlobGasUsed,
		ExcessBlobGas:    &executionPayload.ExcessBlobGas,
		ExecutionWitness: nil,
	}, nil, (*common.Hash)(&signedBlockContents.SignedBlock.Message.ParentRoot), make([][]byte, 0))
	if err != nil {
		t.Fatal(err)
	}
	_ = b
}
