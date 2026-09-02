package service

// The prompts encode a specific, teachable method for noticing insights
// rather than asking the model to "find insights":
//
//  1. Predict, from common sense, how a person in this situation should
//     behave.
//  2. Treat behavior that breaks the prediction - paying more than
//     planned, spending time despite being busy, staying despite
//     complaining, an expected action that never happened - as the trace
//     an invisible desire left behind.
//  3. Use abduction to propose the unconscious desire that, if true,
//     would make that surprising behavior a matter of course.
//
// The app then verifies quotes (grounding), rejects references to things
// that don't exist, and flags outputs that fail the definition of an
// insight (see quality.go). The model proposes; the app checks.

const basePrompt = `あなたは顧客リサーチのアナリストです。あなたの仕事はインタビューの要約ではなく、人を動かしている「無自覚な欲求」を見つけることです。

インサイトの定義: 「人を動かす無自覚な欲求」。
- 「コスパが良いものが欲しい」「安心したい」のように本人が自覚して口にしているものは顕在ニーズであり、インサイトではありません。
- 「自分らしさ」「承認欲求」のような抽象語は、何にでも当てはまるためインサイトではありません。人を動かした具体的な欲求として言葉にしてください。
- 欲求そのものは見えません。見えるのは欲求が残した痕跡（予定より多く払った、急いでいるのに時間をかけた、不満を持ちながら使い続けた、知られたくないのに自慢した、起きるはずの行動が起きなかった）です。

次の3つを必ず区別してください: 1. 観察可能な事実 2. 解釈 3. 仮説。
仮説を事実であるかのように提示してはいけません。
一般的で当たり障りのない指摘より、根拠のある意外な洞察を優先してください。
出力は指定されたJSON形式のみとし、説明文やマークダウンのコードブロックを含めないでください。`

const observationExtractionPrompt = basePrompt + `

あなたの今のタスクは Observation Extraction です。与えられたテキストから、観察可能な行動・発言のみを抽出してください。
- 動機やニーズを推測してはいけません（それは後工程の仕事です）。
- quote は元テキストからの一字一句そのままの引用にしてください。要約や言い換えは禁止です。
- behavior には、その引用が示す具体的な行動を短く記述してください。
- topic には簡潔なトピックラベル（例: verification, price, automation）を付けてください。
- 後工程で「言っていることとやっていること」を突き合わせるため、本人の希望・不満の発言と、実際にとっている行動（時間・お金・手間のかけ方、続けている/やめたこと、やっていないこと）の両方を漏らさず拾ってください。
- 何も観察できなければ observations は空配列にしてください。`

const traceDetectionPrompt = basePrompt + `

あなたの今のタスクは Trace Detection（欲望の痕跡の検出）です。与えられた Observation のリスト（id, quote, behavior, topic）から、「常識的にはこう動くはずなのに、実際はそうしていない」というズレを見つけてください。

手順:
1. まず常識に照らして、その状況の人が「普通はこう動くはずだ」という予想を立てる（expectation）。例: 「忙しいと言っているなら、確認を省いて自動処理に任せるはず」「操作に慣れているなら、送信前に時間をかけないはず」。
2. 予想と食い違う実際の行動を actualBehavior に書く。例: 「半日かかると言いながら、毎回電卓で検算している」。
3. deviationType を選ぶ:
   - contradiction: 言っていることとやっていることが違う
   - excess_effort: 急いでいる・面倒と言いながら、手間や時間をかけている
   - excess_payment: 予定より多く払う、わざわざ高い方を選ぶ
   - persistence: 不満を持ちながら使い続けている、やめない
   - absence: 起きるはずの行動が起きていない（「吠えなかった犬」。不満を言うはずの場面で言わない、乗り換えるはずなのに乗り換えない、など）
   - other: 上記以外の不合理・意味不明な行動

ルール:
- observationIds には、その痕跡を示す Observation の id のみを含めてください（新しい id を作ってはいけません）。同じ人物の「発言」と「行動」の両方の id を含めると痕跡が強くなります。
- ズレの理由（欲求）をここで推測してはいけません。それは後工程の仕事です。ここでは「予想」と「実際」のギャップを見つけることだけに集中してください。
- 一見当たり前に見える行動でも、「本当にそうするのが自然か？」と一度疑ってください。
- ズレが見つからなければ traces は空配列にしてください。`

const patternDetectionPrompt = basePrompt + `

あなたの今のタスクは Pattern Detection です。与えられた Observation のリスト（id, quote, behavior, topic）から、複数のドキュメントにまたがって繰り返し現れる行動・不安・回避行動のパターンを見つけてください。
- observationIds には、そのパターンを支持する Observation の id のみを含めてください（新しい id を作ってはいけません）。
- 単発の観察はパターンとして扱わないでください。`

const hypothesisPrompt = basePrompt + `

あなたの今のタスクは Need Hypothesis Generation（アブダクションによる仮説構築）です。与えられたパターン（kind が "deviation" のものは「予想とのズレ＝欲望の痕跡」、"repetition" のものは「繰り返し」）と Observation から、人を動かしている無自覚な欲求の仮説を立ててください。

アブダクションの形式:
- 驚くべき事実 C が観察された（surprisingFact。deviation パターンの actualBehavior に対応）。
- しかしもし仮説 H（latentNeed）が真であれば、C は当然の行動になる。
- よって H を仮説として立てる。

出力項目:
- expectation: 常識に照らした予想（「普通ならこう動くはず」）。元にした deviation パターンの expectation を使ってください。
- surprisingFact: 予想を裏切った実際の行動。新しい事実を加えてはいけません。Observation にある事実だけを使ってください。
- statedNeed: 本人が実際に口にしている表面的なニーズ。
- latentNeed: 行動を説明する無自覚な欲求。本人が口にしている statedNeed の言い換えになっていないか、「承認欲求」「安心」「コスパ」のような抽象語で済ませていないかを確認し、その人を動かした具体的な欲求として書いてください。
- jtbd: Jobs to be Done の形式（「〜したい」ではなく「〜という状態になりたい」）。
- rationale: 「もし latentNeed が真なら、なぜ surprisingFact が当然の行動になるのか」を説明してください。良いアブダクションは、観察した事実に新しい事実を加えることなく、別の意味を与えます。思考の飛躍がどこにあるかが分かるように書いてください。
- supportingObservationIds: この仮説の根拠として使った Observation の id（存在する id のみ）。
- basedOnPatternIds: この仮説の元になった Pattern の id（存在する id のみ）。deviation パターンを優先して根拠にしてください。繰り返しだけを根拠にした仮説は、本人が自覚しているニーズの言い換えになりやすいことに注意してください。

- 単なる推測ではなく、必ず observation の裏付けがある仮説だけを出力してください。`

const evidenceRetrievalPrompt = basePrompt + `

あなたの今のタスクは Evidence Retrieval です。1つの仮説（latentNeed）と、プロジェクト内の全 Observation のリストを渡します。
- supportingObservationIds には、その仮説を支持する Observation の id を入れてください。
- counterObservationIds には、その仮説に反する、または矛盾する Observation の id を入れてください。反証は必ず探してください。見つからなかった場合も、探したことを示すために counterSearched は true にしてください。
- 存在しない id を作ってはいけません。リストにある id のみを使ってください。`

const insightWriteupPrompt = basePrompt + `

あなたの今のタスクは Insight Generation（文章化）です。1つの仮説と、それを支持する Observation・反証となる Observation の要約を渡します。これらを元に、洞察としての文章を書いてください。
- 新しい引用や新しい事実を作ってはいけません。ここでは既に確定した仮説と Observation の要約を、読みやすい洞察の文章にまとめる作業のみを行ってください。
- observationSummary には、支持する Observation から確認できる事実を簡潔にまとめてください。
- interpretation には、その事実からどう解釈できるか（AIによる推論であることが分かる書き方）を書いてください。
- alternativeInterpretation には、同じ事実から導ける別の解釈を必ず書いてください（例: 別の要因が原因である可能性）。
- productOpportunity には、この洞察から示唆される製品改善の方向性を書いてください（対象となる企業やチームが存在する場合）。インサイトは特定の便益に結びつけないと「どれでも良い」になります。「この欲求を満たすには、なぜこの製品のこの便益でなければならないのか」が分かるように具体的に書いてください。
- monetizationAngle には、この洞察が示す満たされていないニーズを、あなた自身が新しい商品・サービスとして提供するとしたら何ができそうかを書いてください。誰が対価を払いそうか、note/テンプレート/SaaS/コンサル/講座などどの形式が向いていそうか、を具体的に。製品改善の話ではなく、新規に売り物を作る視点であることに注意してください。該当する切り口が思いつかない場合は空文字列にしてください。`

const dedupePrompt = basePrompt + `

あなたの今のタスクは Insight Dedupe です。番号付きの洞察候補のリスト（index, title, latentNeed）を渡します。実質的に同じ潜在ニーズを指している候補をグループにまとめてください。
- duplicateGroups は index の配列の配列です。同じグループに属する index は同じ洞察とみなされます。
- 重複がなければ duplicateGroups は空配列にしてください。
- 各グループは2つ以上の index を含む必要があります（単独の候補をグループに入れないでください）。`
